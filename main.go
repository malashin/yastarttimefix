package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/malashin/ffinfo"
)

// Input file must have "UUID\tURL" structure.

var inputPath string = "input.txt"

var re *regexp.Regexp = regexp.MustCompile(`([a-z0-9]{32})\t(.+)`)

type UuidURL struct {
	UUID string
	URL  string
}

type File struct {
	UuidURL
	Probe ffinfo.File
}

type Data struct {
	File
	FormatStartTime float64
	StreamStartTime []float64
	NonZero         bool
}

func main() {
	// Read input file.
	lines, err := readLines(inputPath)
	if err != nil {
		panic(err)
	}

	total := len(lines)

	for i, line := range lines {
		fmt.Printf("%v/%v: ", i+1, total)

		id, err := parseLine(line)
		if err != nil {
			panic(err)
		}

		fmt.Printf("%v ", id.URL)

		f := File{
			UuidURL: id,
		}

		p, err := ffinfo.Probe(f.URL)
		if err != nil {
			panic(err)
		}

		f.Probe = *p

		d, err := getStartTimes(f)
		if err != nil {
			panic(err)
		}

		if d.NonZero {
			fmt.Printf("%v %v\n", d.FormatStartTime, d.StreamStartTime)

			switch {
			case len(d.StreamStartTime) == 2:
				switch {
				case d.FormatStartTime == 0 && d.StreamStartTime[0] == 0 && d.StreamStartTime[1] > 0 && d.Probe.Streams[0].CodecType == "video" && d.Probe.Streams[1].CodecType == "audio" && d.Probe.Streams[1].Channels == 2:
					command := []string{
						"-i", d.URL,
						"-map", "0:0",
						"-vcodec", "copy",
						"-map", "0:1",
						"-af", fmt.Sprintf("asetpts=PTS-STARTPTS,adelay=delays=%v|%v,apad", d.StreamStartTime[1]*1000, d.StreamStartTime[1]*1000),
						"-acodec", "aac",
						"-ab", "256k",
						"-ar", "48000",
						"-shortest",
						"-loglevel", "error",
						"-stats",
						"-y",
						"-hide_banner",
						strings.TrimSuffix(filepath.Base(d.Probe.Format.Filename), filepath.Ext(d.Probe.Format.Filename)) + "_ptsfix.mp4",
					}

					fmt.Printf("\nffmpeg %v\n\n", strings.Join(command, " "))

					cmd := exec.Command("ffmpeg", command...)
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					if err != nil {
						panic(err)
					}
				}
			}
		}

		fmt.Print("\n")
	}
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func parseLine(line string) (id UuidURL, err error) {
	if !re.MatchString(line) {
		return id, fmt.Errorf("input line does not match \"UUDI\tURL\" pattern: %v", line)
	}

	match := re.FindStringSubmatch(line)

	id = UuidURL{
		UUID: match[1],
		URL:  match[2],
	}

	return id, nil
}

func getStartTimes(f File) (d Data, err error) {
	d.File = f

	d.FormatStartTime, err = strconv.ParseFloat(d.File.Probe.Format.StartTime, 64)
	if err != nil {
		return d, err
	}

	if d.FormatStartTime > 0 {
		d.NonZero = true
	}

	for _, s := range d.File.Probe.Streams {
		streamStartTime, err := strconv.ParseFloat(s.StartTime, 64)
		if err != nil {
			return d, err
		}

		d.StreamStartTime = append(d.StreamStartTime, streamStartTime)

		if streamStartTime > 0 {
			d.NonZero = true
		}
	}

	return d, nil
}
