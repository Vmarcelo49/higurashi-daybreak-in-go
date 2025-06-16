package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Go port of https://github.com/HigurashiArchive/higurashi-daybreak/blob/master/bundle-tools.pl

func main() {
	// Custom usage message
	usage := func() {
		fmt.Println("Usage:")
		fmt.Printf("  %s                                 (Launch GUI mode)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile>                       (Launch GUI mode with DAT file loaded)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s -gui                            (Launch GUI mode)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -list                 (Command line: List content)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -extract <output_folder> [-pattern <files_pattern>] (Command line: Extract images as BMP)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -update <source_files_path> (Command line: Update)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -single-patch <input_file>:<index> (Command line: Patch single file)\n", filepath.Base(os.Args[0]))
		fmt.Println("  (Note: update and patch operations create backups of the original .DAT file before patching)")
	}
	// Handle arguments manually for the correct syntax
	args := os.Args[1:]

	// Check for GUI mode first
	if len(args) == 0 {
		// No arguments - launch GUI
		gui := NewGUI("")
		gui.Run()
		return
	}

	if len(args) == 1 && args[0] == "-gui" {
		// Explicit GUI mode
		gui := NewGUI("")
		gui.Run()
		return
	}

	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		// Single DAT file argument - launch GUI with file loaded
		gui := NewGUI(args[0])
		gui.Run()
		return
	}

	// Command line mode - expect: <datfile> <command> [options]
	if len(args) < 2 {
		fmt.Println("Error: You must provide a DAT file and a command for command line operations")
		usage()
		os.Exit(1)
	}

	datFile := args[0]
	if strings.HasPrefix(datFile, "-") {
		fmt.Println("Error: First argument must be a DAT file, not a flag")
		usage()
		os.Exit(1)
	}

	command := args[1]
	commandArgs := args[2:]

	// Parse commands
	switch command {
	case "-list":
		err := listBundle(datFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "-extract":
		if len(commandArgs) < 1 {
			fmt.Println("Error: -extract requires an output folder")
			usage()
			os.Exit(1)
		}
		outputFolder := commandArgs[0]
		pattern := ""

		// Check for optional -pattern flag
		if len(commandArgs) >= 3 && commandArgs[1] == "-pattern" {
			pattern = commandArgs[2]
		}

		err := extractBundle(datFile, outputFolder, pattern)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case "-update":
		if len(commandArgs) < 1 {
			fmt.Println("Error: -update requires a source files path")
			usage()
			os.Exit(1)
		}
		sourceFilesPath := commandArgs[0]
		patchBundle(datFile, sourceFilesPath)

	case "-single-patch":
		if len(commandArgs) < 1 {
			fmt.Println("Error: -single-patch requires a file:index parameter")
			usage()
			os.Exit(1)
		}

		// Parse the single-patch parameter (format: <file>:<index>)
		parts := strings.Split(commandArgs[0], ":")
		if len(parts) != 2 {
			fmt.Println("Error: Invalid format for -single-patch. Expected <input_file>:<index>")
			os.Exit(1)
		}

		inputFilePath := parts[0]
		indexStr := parts[1]

		// Convert the index string to an integer
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			fmt.Printf("Error: Invalid index '%s'. Must be a number.\n", indexStr)
			os.Exit(1)
		}
		// Patch the single file
		err = patchSingleFile(datFile, inputFilePath, index)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Error: Unknown command '%s'. Must be one of -list, -extract, -update, or -single-patch\n", command)
		usage()
		os.Exit(1)
	}
}
