package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
)

type cmdlineArgs struct {
	SettingsOut bool // Output information about settings at the end
	InFile      string
	OutFile     string
}

var cfg = cmdlineArgs{
	SettingsOut: false,
	InFile:      "", // Use stdin if not set
	OutFile:     "", // Use stdout if not set
}

// FontType - Supported font types
type FontType int

const (
	picaFont FontType = iota
	boldFont
	condensedFont
	italicFont
	eliteFont
)

type documentSettings struct {
	MarginTop        int
	MarginBottom     int
	MarginLeft       int
	MarginRight      int
	MarginLeft2      int
	MarginRight2     int
	PageLength       int
	Indent           int
	Font             FontType
	HeaderCapture    bool
	Header           []byte
	FooterCapture    bool
	Footer           []byte
	Center           bool
	BlockRight       bool
	Justified        bool
	StartPageNum     int
	LineSpacing      int
	ParagraphSpacing int
	SectionLevel     int
	ChainFile        []byte
}

/* printDocumentSettings displays the document settings */
func printDocumentSettings(settings *documentSettings) {
	fmt.Println("\n\nDocument Settings\n=================")
	fmt.Printf("Margins:\n    Top       : %d\n    Bottom    : %d\n    Left      : %d\n    Right     : %d\n\n",
		settings.MarginTop, settings.MarginBottom, settings.MarginLeft, settings.MarginRight)
	fmt.Printf("Column2:\n    Left      : %d\n    Right     : %d\n\n",
		settings.MarginLeft2, settings.MarginRight2)
	fmt.Printf("Page Length   : %d\n", settings.PageLength)
	fmt.Printf("Starting Page : %d\n\n", settings.StartPageNum)
	fmt.Printf("Header        : %s\n", settings.Header)
	fmt.Printf("Footer        : %s\n\n", settings.Footer)
	fmt.Println("Spacing")
	fmt.Printf("    Line      : %d\n", settings.LineSpacing)
	fmt.Printf("    Paragraph : %d\n\n", settings.ParagraphSpacing)
	fmt.Printf("Chained file  : %s\n", settings.ChainFile)
}

/* parseArgs handles parsing the cmdline args and setting values in the global cfg struct */
func parseArgs() {
	flag.BoolVar(&cfg.SettingsOut, "settings", cfg.SettingsOut, "Output settings at the end")
	flag.StringVar(&cfg.InFile, "input", cfg.InFile, "Input file (default stdin)")
	flag.StringVar(&cfg.OutFile, "output", cfg.OutFile, "Output file (default stdout)")

	flag.Parse()
}

/* readUntil reads bytes until the expected string is matched */
func readUntil(fin *bufio.Reader, match []byte) error {
	mIdx := 0
	mBuff := make([]byte, 1)
	for mIdx < len(match) {
		n, err := fin.Read(mBuff)
		if err != nil {
			return err
		}
		if n == 0 {
			// TODO Display how much didn't match
			return errors.New("Input ended too early, no match found")
		}
		if mBuff[0] == match[mIdx] {
			mIdx = mIdx + 1
		} else {
			// Wrong character, reset.
			mIdx = 0
		}
	}
	return nil
}

/* readInt reads a number of ASCII digits and returns them as an int */
func readInt(fin *bufio.Reader, n int) (int, error) {
	buf := make([]byte, n)
	nRead, err := io.ReadFull(fin, buf)
	if err != nil {
		return 0, err
	}
	if nRead != n {
		return 0, fmt.Errorf("ERROR: readInt only read %d byte, not %d as expected", nRead, n)
	}
	value, err := strconv.Atoi(strings.TrimSpace(string(buf)))
	if err != nil {
		return 0, err
	}

	return value, nil
}

/* readString reads characters until it hits a terminator byte */
func readString(fin *bufio.Reader, terminate byte) ([]byte, error) {
	buf := make([]byte, 80)
	mBuff := make([]byte, 1)
	for {
		n, err := io.ReadFull(fin, mBuff)
		if err != nil {
			return nil, err
		}
		if n != 1 {
			return nil, fmt.Errorf("ERROR: readString only read %d byte, not 1 as expected", n)
		}
		if mBuff[0] == terminate {
			break
		}
		buf = append(buf, mBuff[0])
	}
	return buf, nil
}

/* convertStw reads a STWriter document and outputs an ASCII document */
func convertStw(inDoc *bufio.Reader, outDoc *bufio.Writer) error {
	var settings documentSettings
	var nextByte byte
	var err error

	// This *has* to come first
	log.Println("Searching for STWriter file header")
	if err = readUntil(inDoc, []byte("Do Run Run STWRITER.PRG\x00")); err != nil {
		log.Fatal("Did not find STWriter file header")
	}

	for {
		// How to order this? read bytes in state? Process state in byte parsing?

		if nextByte, err = inDoc.ReadByte(); err != nil {
			break
		}

		/*
			0x02 Ctrl-B  Bottom Margin
						 3 bytes '12 '
			0x03 Ctrl-C  Center following text
						 0 bytes
						 2 Ctrl-C == Block Right line of text
			0x04 Ctrl-D  Paragraph Spacing
						 2 bytes '4 '
			0x05 Ctrl-E  Page Eject
			0x06 Ctrl-F  Footer
						 Followed by footer line, @ in footer is replaced by page #
						 2x Ctrl-F turns off footers
			0x07 Ctrl-G  Font Change (0=pica, 1=bold, 2=condensed, 4=italics, 5=elite)
						 2 bytes '0 '
			0x08 Ctrl-H  Header
						 2x Ctrl-H turns off headers
			0x09 Ctrl-I  Paragraph Indentation
						 2 bytes '5 '
			0x0a Ctrl-J  Justification Toggle
						 2 bytes '0 '
			0x0b Ctrl-K  Comment until end of line
			0x0c Ctrl-L  Left Margin
						 3 bytes '10 '
			0x0d Ctrl-M  2 column Left Margin
			0x0e Ctrl-N  2 column Right Margin
			0x0f Ctrl-O  Printer control code
						 3 bytes '15 '
			0x10 Ctrl-P  Paragraph
			0x11 Ctrl-Q  Page # to start with
						 3 bytes (can be negative)
			0x12 Ctrl-R  Right Margin
						 3 bytes '70 '
			0x13 Ctrl-S  Line Spacing
						 1 byte '2'
			0x14 Ctrl-T  Top margin
						 3 bytes '12 '
			0x15 Ctrl-U  Section Heading Level
						 1 byte
			0x16 Ctrl-V  Link file, followed by path and filename
						 Read until end of line
			0x17 Ctrl-W  Page Wait
			0x18 Ctrl-X  Escape printer codes, ended by Ctrl-X
			0x19 Ctrl-Y  Lines Per Page
						 Followed by 3 bytes of ASCII (eg. '132')
			0x1a Ctrl-Z  Unused
		*/
		// Check for control codes
		switch nextByte {
		case 0x00: // End of a line/paragraph
			outDoc.WriteByte('\n')

			// Turn off line oriented flags
			settings.Center = false
			settings.BlockRight = false
		case 0x02: // Set the Bottom Margin
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.MarginBottom = value
			}
		case 0x03: // Center or Block Right until end of line
			if settings.Center {
				settings.Center = false
				settings.BlockRight = true
			} else {
				settings.Center = true
			}
		case 0x04: // Paragraph spacing
			value, err := readInt(inDoc, 2)
			if err != nil {
				log.Println(err)
			} else {
				settings.ParagraphSpacing = value
			}
		case 0x05: // Page Eject
			// Ignore
		case 0x06: // Footer
			if settings.FooterCapture {
				settings.FooterCapture = false
				log.Printf("FOOTER: %s", settings.Footer)
			} else {
				settings.FooterCapture = true
				settings.Footer = make([]byte, 80)
			}
		case 0x07: // Font change
			value, err := readInt(inDoc, 2)
			if err != nil {
				log.Println(err)
			} else {
				settings.Font = FontType(value)
			}
		case 0x08: // Header
			if settings.HeaderCapture {
				settings.HeaderCapture = false
				log.Printf("HEADER: %s", settings.Header)
			} else {
				settings.HeaderCapture = true
				settings.Header = make([]byte, 80)
			}
		case 0x09: // Paragraph Indent
			value, err := readInt(inDoc, 2)
			if err != nil {
				log.Println(err)
			} else {
				settings.Indent = value
			}
		case 0x0a: // Justification toggle
			value, err := readInt(inDoc, 2)
			if err != nil {
				log.Println(err)
			} else {
				if value == 1 {
					settings.Justified = true
				} else {
					settings.Justified = false
				}
			}
		case 0x0b: // Comment until end of line
			outDoc.Write([]byte("COMMENT: "))
		case 0x0c: // Left Margin
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.MarginLeft = value
			}
		case 0x0d: // Column2 Left Margin
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.MarginLeft2 = value
			}
		case 0x0e: // Column2 Left Margin
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.MarginRight2 = value
			}
		case 0x0f: // Printer Control Code
			// Read it and ignore it
			_, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			}
		case 0x10: // Paragraph
			outDoc.Write([]byte("\n\n"))
		case 0x11: // Starting page number
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.StartPageNum = value
			}
		case 0x12: // Right Margin
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.MarginRight = value
			}
		case 0x13: // Line spacing
			value, err := readInt(inDoc, 1)
			if err != nil {
				log.Println(err)
			} else {
				settings.LineSpacing = value
			}
		case 0x14: // Line spacing
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.MarginTop = value
			}
		case 0x15: // Section Heading Level
			value, err := readInt(inDoc, 1)
			if err != nil {
				log.Println(err)
			} else {
				settings.SectionLevel = value
			}
		case 0x16: // Chain filename
			filename, err := readString(inDoc, 0x00)
			if err != nil {
				log.Println(err)
			} else {
				copy(settings.ChainFile, filename)
			}
		case 0x17: // Page Wait
			// Ignore
		case 0x18: // Escape Printer Control Codes
			// Read until another 0x18
			_, err := readString(inDoc, 0x18)
			if err != nil {
				log.Println(err)
			}
		case 0x19: // Lines per page
			value, err := readInt(inDoc, 3)
			if err != nil {
				log.Println(err)
			} else {
				settings.PageLength = value
			}
		default:
			// Skip any unprintable bytes that have slipped through
			if !strconv.IsPrint(rune(nextByte)) {
				break
			}
			if settings.FooterCapture {
				// Capture the footer
				settings.Footer = append(settings.Footer, nextByte)
			} else if settings.HeaderCapture {
				// Capture the header
				settings.Header = append(settings.Header, nextByte)
			} else {
				outDoc.WriteByte(nextByte)
			}
		}
	}
	outDoc.Flush()

	if cfg.SettingsOut {
		printDocumentSettings(&settings)
	}

	return nil
}

/* main sets up the input and output files, calls convertStw */
func main() {
	parseArgs()

	var fin, fout *os.File
	var err error
	if len(cfg.InFile) > 0 {
		if fin, err = os.Open(cfg.InFile); err != nil {
			log.Fatal(err)
		}
		defer fin.Close()
	} else {
		fin = os.Stdin
	}

	if len(cfg.OutFile) > 0 {
		if fout, err = os.Create(cfg.OutFile); err != nil {
			log.Fatal(err)
		}
		defer fout.Close()
	} else {
		fout = os.Stdout
	}

	inDoc := bufio.NewReader(fin)
	outDoc := bufio.NewWriter(fout)
	if err = convertStw(inDoc, outDoc); err != nil {
		log.Fatal(err)
	}
}
