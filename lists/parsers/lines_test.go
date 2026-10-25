package parsers

import (
	"bufio"
	"context"
	"io"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lines", func() {
	var (
		data string

		sutReader io.Reader
		sut       SeriesParser[string]
	)

	BeforeEach(func() {
		sutReader = nil
	})

	JustBeforeEach(func() {
		if sutReader == nil {
			sutReader = strings.NewReader(data)
		}

		sut = Lines(sutReader)
	})

	When("it has normal lines", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"first",
				"second",
				"third",
			)
		})

		It("returns them all", func() {
			str, err := sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("first"))
			Expect(sut.Position()).Should(Equal("line 1"))

			str, err = sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("second"))
			Expect(sut.Position()).Should(Equal("line 2"))

			str, err = sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("third"))
			Expect(sut.Position()).Should(Equal("line 3"))

			_, err = sut.Next(context.Background())
			Expect(err).Should(HaveOccurred())
			Expect(err).Should(MatchError(io.EOF))
			Expect(IsNonResumableErr(err)).Should(BeTrue())
			Expect(sut.Position()).Should(Equal("line 4"))
		})
	})

	When("it has empty lines", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"",
				"  ",
				"\t",
				"\r",
			)
		})

		It("skips them", func() {
			_, err := sut.Next(context.Background())
			Expect(err).Should(HaveOccurred())
			Expect(err).Should(MatchError(io.EOF))
			Expect(IsNonResumableErr(err)).Should(BeTrue())
			Expect(sut.Position()).Should(Equal("line 5"))
		})
	})

	When("it has commented lines", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"first",
				"# comment 1",
				"# comment 2",
				"second",
			)
		})

		It("returns them all", func() {
			str, err := sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("first"))
			Expect(sut.Position()).Should(Equal("line 1"))

			str, err = sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("second"))
			Expect(sut.Position()).Should(Equal("line 4"))
		})
	})

	When("it has end of line comments", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"first# comment",
				"second # other",
			)
		})

		It("returns them all", func() {
			str, err := sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("first"))
			Expect(sut.Position()).Should(Equal("line 1"))

			str, err = sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("second"))
			Expect(sut.Position()).Should(Equal("line 2"))
		})
	})

	When("there's a scan error", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"too long " + strings.Repeat(".", maxScannerBuffer),
			)
		})

		It("fails", func() {
			_, err := sut.Next(context.Background())
			Expect(err).Should(HaveOccurred())
			Expect(sut.Position()).Should(Equal("line 1"))
		})
	})

	When("a line is longer than the bufio.Scanner default of 64 KiB", func() {
		// Such a line used to abort the whole parse with a non-resumable error,
		// silently leaving the group empty and thus blocking nothing.
		longLine := "long." + strings.Repeat("a", 2*bufio.MaxScanTokenSize) + ".example"

		BeforeEach(func() {
			sutReader = linesReader(longLine, "after.example")
		})

		It("still yields it and the following lines", func() {
			str, err := sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal(longLine))

			str, err = sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("after.example"))
		})
	})

	When("context is cancelled", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"first",
				"second",
			)
		})

		It("stops parsing", func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			str, err := sut.Next(ctx)
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("first"))
			Expect(sut.Position()).Should(Equal("line 1"))

			cancel()

			_, err = sut.Next(ctx)
			Expect(err).Should(HaveOccurred())
			Expect(IsNonResumableErr(err)).Should(BeTrue())
			Expect(sut.Position()).Should(Equal("line 2"))
		})
	})

	When("last line has no newline", func() {
		BeforeEach(func() {
			data = "first\nlast"
		})

		It("still returns it", func() {
			str, err := sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("first"))
			Expect(sut.Position()).Should(Equal("line 1"))

			str, err = sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("last"))
			Expect(sut.Position()).Should(Equal("line 2"))
		})
	})

	When("it has Adblock identifier or comments", func() {
		BeforeEach(func() {
			sutReader = linesReader(
				"[Adblock",
				"! comment 1",
				"! comment 2",
				"second",
			)
		})

		It("skips them", func() {
			str, err := sut.Next(context.Background())
			Expect(err).Should(Succeed())
			Expect(str).Should(Equal("second"))
			Expect(sut.Position()).Should(Equal("line 4"))
		})
	})
})

func linesReader(lines ...string) io.Reader {
	data := strings.Join(lines, "\n") + "\n"

	return strings.NewReader(data)
}
