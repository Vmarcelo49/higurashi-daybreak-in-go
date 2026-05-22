package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"bundleTools/archive"
)

// Go port of https://github.com/HigurashiArchive/higurashi-daybreak/blob/master/bundle-tools.pl

func main() {
	// CLI parsing and execution are split into helper functions to
	// keep `main` minimal and centralize error handling.
	datFile, opts, err := parseArgs()
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(1)
	}

	if err := run(datFile, opts); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Options holds CLI flag values after parsing.
type Options struct {
	List             bool
	Extract          string
	Pattern          string
	ExtractSingle    int
	ExtractSingleOut string
	Update           string
	SinglePatch      string
}

// parseArgs configures flags, parses os.Args and returns the dat file and options.
func parseArgs() (string, Options, error) {
	usage := func() {
		fmt.Println("Usage:")
		fmt.Printf("  %s <datfile> -list                                     (List content)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -extract <output_folder> [-pattern <files_pattern>]\n", filepath.Base(os.Args[0]))
		fmt.Printf("      (Extract files to output folder)\n")
		fmt.Printf("  %s <datfile> -extract-single <index> -out <output_file>\n", filepath.Base(os.Args[0]))
		fmt.Printf("      (Extract single file by index)\n")
		fmt.Printf("  %s <datfile> -update <source_files_path>               (Update from source path)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -single-patch <input_file>:<index>        (Patch single file)\n", filepath.Base(os.Args[0]))
		fmt.Println("  (Note: update and patch operations create backups of the original .DAT file before patching)")
	}

	listFlag := flag.Bool("list", false, "List content of the DAT file")
	extractFlag := flag.String("extract", "", "Extract to output folder")
	patternFlag := flag.String("pattern", "", "File pattern to filter when extracting")
	extractSingleFlag := flag.Int("extract-single", -1, "Extract single file by index")
	extractSingleOut := flag.String("out", "", "Output file for -extract-single")
	updateFlag := flag.String("update", "", "Source files path to update the DAT")
	singlePatchFlag := flag.String("single-patch", "", "Patch single file using format input_file:index")

	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		return "", Options{}, fmt.Errorf("you must provide a DAT file")
	}

	datFile := args[0]

	opts := Options{
		List:             *listFlag,
		Extract:          *extractFlag,
		Pattern:          *patternFlag,
		ExtractSingle:    *extractSingleFlag,
		ExtractSingleOut: *extractSingleOut,
		Update:           *updateFlag,
		SinglePatch:      *singlePatchFlag,
	}

	return datFile, opts, nil
}

// run executes the requested command and returns an error instead of exiting.
func run(datFile string, opts Options) error {
	switch {
	case opts.List:
		return archive.ListBundle(datFile)

	case opts.Extract != "":
		return archive.ExtractBundle(datFile, opts.Extract, opts.Pattern)

	case opts.ExtractSingle >= 0:
		if opts.ExtractSingleOut == "" {
			return fmt.Errorf("-extract-single requires -out <output_file>")
		}
		if err := archive.ExtractSingleFile(datFile, opts.ExtractSingle, opts.ExtractSingleOut); err != nil {
			return err
		}
		fmt.Printf("File at index %d extracted successfully to: %s\n", opts.ExtractSingle, opts.ExtractSingleOut)
		return nil

	case opts.Update != "":
		archive.PatchBundle(datFile, opts.Update)
		return nil

	case opts.SinglePatch != "":
		parts := strings.Split(opts.SinglePatch, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid format for -single-patch. Expected <input_file>:<index>")
		}
		inputFilePath := parts[0]
		indexStr := parts[1]
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			return fmt.Errorf("invalid index '%s'. Must be a number", indexStr)
		}
		return archive.PatchSingleFile(datFile, inputFilePath, index)

	default:
		return fmt.Errorf("no command provided")
	}
}
