package color

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/mattn/go-colorable"
)

const expectedRedFooOutput = "\x1b[31mfoo\x1b[0m\n"

// Testing colors is kinda different. First we test for given colors and their
// escaped formatted results. Next we create some visual tests to be tested.
// Each visual test includes the color name to be compared.
func TestColor(t *testing.T) {
	rb := new(bytes.Buffer)
	Output = rb

	NoColor = false

	testColors := []struct {
		text string
		code Attribute
	}{
		{text: "black", code: FgBlack},
		{text: "red", code: FgRed},
		{text: "green", code: FgGreen},
		{text: "yellow", code: FgYellow},
		{text: "blue", code: FgBlue},
		{text: "magent", code: FgMagenta},
		{text: "cyan", code: FgCyan},
		{text: "white", code: FgWhite},
		{text: "hblack", code: FgHiBlack},
		{text: "hred", code: FgHiRed},
		{text: "hgreen", code: FgHiGreen},
		{text: "hyellow", code: FgHiYellow},
		{text: "hblue", code: FgHiBlue},
		{text: "hmagent", code: FgHiMagenta},
		{text: "hcyan", code: FgHiCyan},
		{text: "hwhite", code: FgHiWhite},
	}

	for _, c := range testColors {
		_, _ = New(c.code).Print(c.text)

		line, _ := rb.ReadString('\n')
		scannedLine := fmt.Sprintf("%q", line)
		colored := fmt.Sprintf("\x1b[%dm%s\x1b[0m", c.code, c.text)
		escapedForm := fmt.Sprintf("%q", colored)

		t.Logf("%s\t: %s", c.text, line)

		if scannedLine != escapedForm {
			t.Errorf("Expecting %s, got '%s'\n", escapedForm, scannedLine)
		}
	}

	for _, c := range testColors {
		line := New(c.code).Sprintf("%s", c.text)
		scannedLine := fmt.Sprintf("%q", line)
		colored := fmt.Sprintf("\x1b[%dm%s\x1b[0m", c.code, c.text)
		escapedForm := fmt.Sprintf("%q", colored)

		t.Logf("%s\t: %s", c.text, line)

		if scannedLine != escapedForm {
			t.Errorf("Expecting %s, got '%s'\n", escapedForm, scannedLine)
		}
	}
}

func TestColorEquals(t *testing.T) {
	fgblack1 := New(FgBlack)
	fgblack2 := New(FgBlack)
	bgblack := New(BgBlack)
	fgbgblack := New(FgBlack, BgBlack)
	fgblackbgred := New(FgBlack, BgRed)
	fgred := New(FgRed)
	bgred := New(BgRed)

	if !fgblack1.Equals(fgblack2) {
		t.Error("Two black colors are not equal")
	}

	if fgblack1.Equals(bgblack) {
		t.Error("Fg and bg black colors are equal")
	}

	if fgblack1.Equals(fgbgblack) {
		t.Error("Fg black equals fg/bg black color")
	}

	if fgblack1.Equals(fgred) {
		t.Error("Fg black equals Fg red")
	}

	if fgblack1.Equals(bgred) {
		t.Error("Fg black equals Bg red")
	}

	if fgblack1.Equals(fgblackbgred) {
		t.Error("Fg black equals fg black bg red")
	}
}

func TestNoColor(t *testing.T) {
	rb := new(bytes.Buffer)
	Output = rb

	testColors := []struct {
		text string
		code Attribute
	}{
		{text: "black", code: FgBlack},
		{text: "red", code: FgRed},
		{text: "green", code: FgGreen},
		{text: "yellow", code: FgYellow},
		{text: "blue", code: FgBlue},
		{text: "magent", code: FgMagenta},
		{text: "cyan", code: FgCyan},
		{text: "white", code: FgWhite},
		{text: "hblack", code: FgHiBlack},
		{text: "hred", code: FgHiRed},
		{text: "hgreen", code: FgHiGreen},
		{text: "hyellow", code: FgHiYellow},
		{text: "hblue", code: FgHiBlue},
		{text: "hmagent", code: FgHiMagenta},
		{text: "hcyan", code: FgHiCyan},
		{text: "hwhite", code: FgHiWhite},
	}

	for _, c := range testColors {
		p := New(c.code)
		p.DisableColor()
		_, _ = p.Print(c.text)

		line, _ := rb.ReadString('\n')
		if line != c.text {
			t.Errorf("Expecting %s, got '%s'\n", c.text, line)
		}
	}

	// global check
	NoColor = true

	t.Cleanup(func() {
		NoColor = false
	})

	for _, c := range testColors {
		p := New(c.code)
		_, _ = p.Print(c.text)

		line, _ := rb.ReadString('\n')
		if line != c.text {
			t.Errorf("Expecting %s, got '%s'\n", c.text, line)
		}
	}
}

func TestNoColor_Env(t *testing.T) {
	rb := new(bytes.Buffer)
	Output = rb

	testColors := []struct {
		text string
		code Attribute
	}{
		{text: "black", code: FgBlack},
		{text: "red", code: FgRed},
		{text: "green", code: FgGreen},
		{text: "yellow", code: FgYellow},
		{text: "blue", code: FgBlue},
		{text: "magent", code: FgMagenta},
		{text: "cyan", code: FgCyan},
		{text: "white", code: FgWhite},
		{text: "hblack", code: FgHiBlack},
		{text: "hred", code: FgHiRed},
		{text: "hgreen", code: FgHiGreen},
		{text: "hyellow", code: FgHiYellow},
		{text: "hblue", code: FgHiBlue},
		{text: "hmagent", code: FgHiMagenta},
		{text: "hcyan", code: FgHiCyan},
		{text: "hwhite", code: FgHiWhite},
	}

	t.Setenv("NO_COLOR", "1")

	t.Cleanup(func() {
		_ = os.Unsetenv("NO_COLOR")
	})

	for _, c := range testColors {
		p := New(c.code)
		_, _ = p.Print(c.text)

		line, _ := rb.ReadString('\n')
		if line != c.text {
			t.Errorf("Expecting %s, got '%s'\n", c.text, line)
		}
	}
}

func Test_noColorIsSet(t *testing.T) {
	tests := []struct {
		name string
		act  func()
		want bool
	}{
		{
			name: "default",
			act:  func() {},
			want: false,
		},
		{
			name: "NO_COLOR=1",
			act:  func() { t.Setenv("NO_COLOR", "1") },
			want: true,
		},
		{
			name: "NO_COLOR=",
			act:  func() { t.Setenv("NO_COLOR", "") },
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() {
				_ = os.Unsetenv("NO_COLOR")
			})
			tt.act()

			if got := noColorIsSet(); got != tt.want {
				t.Errorf("noColorIsSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestColorVisual(t *testing.T) {
	// First Visual Test
	Output = colorable.NewColorableStdout()

	_, _ = New(FgRed).Printf("red\t")
	_, _ = New(BgRed).Print("         ")
	_, _ = New(FgRed, Bold).Println(" red")

	_, _ = New(FgGreen).Printf("green\t")
	_, _ = New(BgGreen).Print("         ")
	_, _ = New(FgGreen, Bold).Println(" green")

	_, _ = New(FgYellow).Printf("yellow\t")
	_, _ = New(BgYellow).Print("         ")
	_, _ = New(FgYellow, Bold).Println(" yellow")

	_, _ = New(FgBlue).Printf("blue\t")
	_, _ = New(BgBlue).Print("         ")
	_, _ = New(FgBlue, Bold).Println(" blue")

	_, _ = New(FgMagenta).Printf("magenta\t")
	_, _ = New(BgMagenta).Print("         ")
	_, _ = New(FgMagenta, Bold).Println(" magenta")

	_, _ = New(FgCyan).Printf("cyan\t")
	_, _ = New(BgCyan).Print("         ")
	_, _ = New(FgCyan, Bold).Println(" cyan")

	_, _ = New(FgWhite).Printf("white\t")
	_, _ = New(BgWhite).Print("         ")
	_, _ = New(FgWhite, Bold).Println(" white")

	t.Log("")

	// Second Visual test
	Blackf("black")
	Redf("red")
	Greenf("green")
	Yellowf("yellow")
	Bluef("blue")
	Magentaf("magenta")
	Cyanf("cyan")
	Whitef("white")
	HiBlackf("hblack")
	HiRedf("hred")
	HiGreenf("hgreen")
	HiYellowf("hyellow")
	HiBluef("hblue")
	HiMagentaf("hmagenta")
	HiCyanf("hcyan")
	HiWhitef("hwhite")

	// Third visual test
	t.Log("")
	Set(FgBlue)
	t.Log("is this blue?")
	Unset()

	Set(FgMagenta)
	t.Log("and this magenta?")
	Unset()

	// Fourth Visual test
	t.Log("")

	blue := New(FgBlue).PrintlnFunc()
	blue("blue text with custom print func")

	red := New(FgRed).PrintfFunc()
	red("red text with a printf func: %d\n", 123)

	put := New(FgYellow).SprintFunc()
	warn := New(FgRed).SprintFunc()

	_, _ = fmt.Fprintf(Output, "this is a %s and this is %s.\n", put("warning"), warn("error"))

	info := New(FgWhite, BgGreen).SprintFunc()
	_, _ = fmt.Fprintf(Output, "this %s rocks!\n", info("package"))

	notice := New(FgBlue).FprintFunc()
	notice(os.Stderr, "just a blue notice to stderr")

	// Fifth Visual Test
	t.Log("")

	_, _ = fmt.Fprintln(Output, BlackString("black"))
	_, _ = fmt.Fprintln(Output, RedString("red"))
	_, _ = fmt.Fprintln(Output, GreenString("green"))
	_, _ = fmt.Fprintln(Output, YellowString("yellow"))
	_, _ = fmt.Fprintln(Output, BlueString("blue"))
	_, _ = fmt.Fprintln(Output, MagentaString("magenta"))
	_, _ = fmt.Fprintln(Output, CyanString("cyan"))
	_, _ = fmt.Fprintln(Output, WhiteString("white"))
	_, _ = fmt.Fprintln(Output, HiBlackString("hblack"))
	_, _ = fmt.Fprintln(Output, HiRedString("hred"))
	_, _ = fmt.Fprintln(Output, HiGreenString("hgreen"))
	_, _ = fmt.Fprintln(Output, HiYellowString("hyellow"))
	_, _ = fmt.Fprintln(Output, HiBlueString("hblue"))
	_, _ = fmt.Fprintln(Output, HiMagentaString("hmagenta"))
	_, _ = fmt.Fprintln(Output, HiCyanString("hcyan"))
	_, _ = fmt.Fprintln(Output, HiWhiteString("hwhite"))
}

func TestNoFormat(t *testing.T) {
	t.Logf("%s   %%s = ", BlackString("Black"))
	Blackf("%s")

	t.Logf("%s     %%s = ", RedString("Red"))
	Redf("%s")

	t.Logf("%s   %%s = ", GreenString("Green"))
	Greenf("%s")

	t.Logf("%s  %%s = ", YellowString("Yellow"))
	Yellowf("%s")

	t.Logf("%s    %%s = ", BlueString("Blue"))
	Bluef("%s")

	t.Logf("%s %%s = ", MagentaString("Magenta"))
	Magentaf("%s")

	t.Logf("%s    %%s = ", CyanString("Cyan"))
	Cyanf("%s")

	t.Logf("%s   %%s = ", WhiteString("White"))
	Whitef("%s")

	t.Logf("%s   %%s = ", HiBlackString("HiBlack"))
	HiBlackf("%s")

	t.Logf("%s     %%s = ", HiRedString("HiRed"))
	HiRedf("%s")

	t.Logf("%s   %%s = ", HiGreenString("HiGreen"))
	HiGreenf("%s")

	t.Logf("%s  %%s = ", HiYellowString("HiYellow"))
	HiYellowf("%s")

	t.Logf("%s    %%s = ", HiBlueString("HiBlue"))
	HiBluef("%s")

	t.Logf("%s %%s = ", HiMagentaString("HiMagenta"))
	HiMagentaf("%s")

	t.Logf("%s    %%s = ", HiCyanString("HiCyan"))
	HiCyanf("%s")

	t.Logf("%s   %%s = ", HiWhiteString("HiWhite"))
	HiWhitef("%s")
}

func TestNoFormatString(t *testing.T) {
	tests := []struct {
		f      func(string, ...any) string
		format string
		args   []any
		want   string
	}{
		{BlackString, "%s", nil, "\x1b[30m%s\x1b[0m"},
		{RedString, "%s", nil, "\x1b[31m%s\x1b[0m"},
		{GreenString, "%s", nil, "\x1b[32m%s\x1b[0m"},
		{YellowString, "%s", nil, "\x1b[33m%s\x1b[0m"},
		{BlueString, "%s", nil, "\x1b[34m%s\x1b[0m"},
		{MagentaString, "%s", nil, "\x1b[35m%s\x1b[0m"},
		{CyanString, "%s", nil, "\x1b[36m%s\x1b[0m"},
		{WhiteString, "%s", nil, "\x1b[37m%s\x1b[0m"},
		{HiBlackString, "%s", nil, "\x1b[90m%s\x1b[0m"},
		{HiRedString, "%s", nil, "\x1b[91m%s\x1b[0m"},
		{HiGreenString, "%s", nil, "\x1b[92m%s\x1b[0m"},
		{HiYellowString, "%s", nil, "\x1b[93m%s\x1b[0m"},
		{HiBlueString, "%s", nil, "\x1b[94m%s\x1b[0m"},
		{HiMagentaString, "%s", nil, "\x1b[95m%s\x1b[0m"},
		{HiCyanString, "%s", nil, "\x1b[96m%s\x1b[0m"},
		{HiWhiteString, "%s", nil, "\x1b[97m%s\x1b[0m"},
	}

	for i, test := range tests {
		s := test.f(test.format, test.args...)

		if s != test.want {
			t.Errorf("[%d] want: %q, got: %q", i, test.want, s)
		}
	}
}

func TestColor_Println_Newline(t *testing.T) {
	rb := new(bytes.Buffer)
	Output = rb

	c := New(FgRed)
	_, _ = c.Println("foo")

	got := readRaw(t, rb)
	want := expectedRedFooOutput

	if want != got {
		t.Errorf("Println newline error\n\nwant: %q\n got: %q", want, got)
	}
}

func TestColor_Sprintln_Newline(t *testing.T) {
	c := New(FgRed)

	got := c.Sprintln("foo")
	want := "\x1b[31mfoo\x1b[0m\n"

	if want != got {
		t.Errorf("Println newline error\n\nwant: %q\n got: %q", want, got)
	}
}

func TestColor_Fprintln_Newline(t *testing.T) {
	rb := new(bytes.Buffer)
	c := New(FgRed)
	_, _ = c.Fprintln(rb, "foo")

	got := readRaw(t, rb)
	want := "\x1b[31mfoo\x1b[0m\n"

	if want != got {
		t.Errorf("Println newline error\n\nwant: %q\n got: %q", want, got)
	}
}

func readRaw(t *testing.T, r io.Reader) string {
	t.Helper()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	return string(out)
}

func TestIssue206_1(t *testing.T) {
	// visual test, go test -v .
	// to  see the string with escape codes, use go test -v . > c:\temp\test.txt
	underline := New(Underline).Sprint

	line := fmt.Sprintf("%s %s %s %s", "word1", underline("word2"), "word3", underline("word4"))

	line = CyanString(line)

	t.Log(line)

	result := line

	const expectedResult = "\x1b[36mword1 \x1b[4mword2\x1b[24m word3 \x1b[4mword4\x1b[24m\x1b[0m"

	if !bytes.Equal([]byte(result), []byte(expectedResult)) {
		t.Errorf("Expecting %v, got '%v'\n", expectedResult, result)
	}
}

func TestIssue206_2(t *testing.T) {
	underline := New(Underline).Sprint
	bold := New(Bold).Sprint

	line := fmt.Sprintf(
		"%s %s",
		GreenString(underline("underlined regular green")),
		RedString(bold("bold red")),
	)

	t.Log(line)

	result := line

	const expectedResult = "\x1b[32m\x1b[4munderlined regular green\x1b[24m\x1b[0m \x1b[31m\x1b[1mbold red\x1b[22m\x1b[0m"

	if !bytes.Equal([]byte(result), []byte(expectedResult)) {
		t.Errorf("Expecting %v, got '%v'\n", expectedResult, result)
	}
}

func TestIssue218(t *testing.T) {
	// Adds a newline to the end of the last string to make sure it isn't trimmed.
	params := []any{"word1", "word2", "word3", "word4\n"}

	c := New(FgCyan)
	_, _ = c.Println(params...)

	result := c.Sprintln(params...)
	t.Logf("params: %v", params)
	t.Log(result)

	const expectedResult = "\x1b[36mword1 word2 word3 word4\n\x1b[0m\n"

	if !bytes.Equal([]byte(result), []byte(expectedResult)) {
		t.Errorf(
			"Sprintln: Expecting %v (%v), got '%v (%v)'\n",
			expectedResult,
			[]byte(expectedResult),
			result,
			[]byte(result),
		)
	}

	fn := c.SprintlnFunc()

	result = fn(params...)
	if !bytes.Equal([]byte(result), []byte(expectedResult)) {
		t.Errorf(
			"SprintlnFunc: Expecting %v (%v), got '%v (%v)'\n",
			expectedResult,
			[]byte(expectedResult),
			result,
			[]byte(result),
		)
	}

	var buf bytes.Buffer

	_, _ = c.Fprintln(&buf, params...)

	result = buf.String()
	if !bytes.Equal([]byte(result), []byte(expectedResult)) {
		t.Errorf(
			"Fprintln: Expecting %v (%v), got '%v (%v)'\n",
			expectedResult,
			[]byte(expectedResult),
			result,
			[]byte(result),
		)
	}
}

func TestRGB(t *testing.T) {
	tests := []struct {
		r, g, b int
	}{
		{255, 128, 0}, // orange
		{230, 42, 42}, // red
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(_ *testing.T) {
			_, _ = RGB(tt.r, tt.g, tt.b).Println("foreground")
			_, _ = RGB(tt.r, tt.g, tt.b).AddBgRGB(0, 0, 0).Println("with background")
			_, _ = BgRGB(tt.r, tt.g, tt.b).Println("background")
			_, _ = BgRGB(tt.r, tt.g, tt.b).AddRGB(255, 255, 255).Println("with foreground")
		})
	}
}
