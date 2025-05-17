package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Go port of https://github.com/HigurashiArchive/higurashi-daybreak/blob/master/bundle-tools.pl

func main() { // Define command-line flags
	listFlag := flag.Bool("list", false, "List contents of the DAT file")
	extractFlag := flag.String("extract", "", "Extract files to the specified output folder")
	updateFlag := flag.String("update", "", "Path to source files directory to patch into the DAT file")
	patternFlag := flag.String("pattern", "", "File pattern to match when extracting, .x would match *.x files")
	guiFlag := flag.Bool("gui", false, "Launch GUI mode")
	singlePatchFlag := flag.String("single-patch", "", "Patch a single file into the DAT (format: <input_file>:<index>)")
	imageFormatFlag := flag.String("image-format", "png", "Format for image conversion: 'tga' or 'png' (default: png)")

	// Add test function flags
	testPatchFlag := flag.String("test-patch", "", "Test patching a single file without format conversion (format: <input_file>:<index>)")
	testExtractFlag := flag.String("test-extract", "", "Test extracting a single file (format: <output_dir>:<index>)")
	testFullCycleFlag := flag.String("test-full-cycle", "", "Test extract, patch, and verify cycle (format: <temp_dir>:<index>)")

	// Define custom usage message
	flag.Usage = func() {
		fmt.Println("Usage:")
		fmt.Printf("  %s                                 (Launch GUI mode)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile>                       (Launch GUI mode with DAT file loaded)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s -gui                            (Launch GUI mode)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -list                 (Command line: List content)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -extract <output_folder> [-pattern <files_pattern>] (Command line: Extract)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -update <source_files_path> (Command line: Update)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -single-patch <input_file>:<index> (Command line: Patch single file)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -test-patch <input_file>:<index> (Test: Patch without conversion)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -test-extract <output_dir>:<index> (Test: Extract single file)\n", filepath.Base(os.Args[0]))
		fmt.Printf("  %s <datfile> -test-full-cycle <temp_dir>:<index> (Test: Extract, patch, verify)\n", filepath.Base(os.Args[0]))
		fmt.Println("  (Note: update and patch operations create backups of the original .DAT file before patching)")
		fmt.Println("\nFlags:")
		flag.PrintDefaults()
	}

	// Parse flags
	flag.Parse()

	// Set image output format based on command line flag
	if *imageFormatFlag == "png" {
		ImageOutputFormat = "png"
	} else {
		ImageOutputFormat = "tga" // Default
	}

	// Get non-flag arguments
	args := flag.Args()
	// Check if GUI mode was requested explicitly or if no flags are provided
	if *guiFlag || (len(args) == 0 && !*listFlag && *extractFlag == "" && *updateFlag == "" && *patternFlag == "" &&
		*singlePatchFlag == "" && *testPatchFlag == "" && *testExtractFlag == "" && *testFullCycleFlag == "") {
		initialFile := ""
		// If a single argument is provided and no flags, use it as the initial file
		if len(args) == 1 {
			initialFile = args[0]
		}
		gui := NewGUI(initialFile)
		gui.Run()
		return
	}

	// Check if a DAT file was provided as first argument for command line operations
	if len(args) < 1 {
		fmt.Println("Error: You must provide a DAT file as the first argument for command line operations")
		flag.Usage()
		os.Exit(1)
	}

	datFile := args[0]

	// Determine which command to run
	switch {
	case *listFlag:
		err := listBundle(datFile)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case *extractFlag != "":
		err := extractBundle(datFile, *extractFlag, *patternFlag)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	case *updateFlag != "":
		patchBundle(datFile, *updateFlag)

	case *singlePatchFlag != "":
		// Parse the single-patch parameter (format: <file>:<index>)
		parts := strings.Split(*singlePatchFlag, ":")
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
	case *testPatchFlag != "":
		// Parse the test-patch parameter (format: <file>:<index>)
		parts := strings.Split(*testPatchFlag, ":")
		if len(parts) != 2 {
			fmt.Println("Error: Invalid format for -test-patch. Expected <input_file>:<index>")
			fmt.Printf("Received: %s\n", *testPatchFlag)
			os.Exit(1)
		}

		inputFilePath := parts[0]
		indexStr := parts[1]

		fmt.Printf("TEST-PATCH: Patching file '%s', index '%s'\n", inputFilePath, indexStr)

		// Convert the index string to an integer
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			fmt.Printf("Error: Invalid index '%s'. Must be a number.\n", indexStr)
			os.Exit(1)
		}

		// Patch the single file using the test function (no format conversion)
		fmt.Println("TEST MODE: Patching without format conversion")
		err = testPatchSingleFile(datFile, inputFilePath, index)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case *testExtractFlag != "":
		// Parse the test-extract parameter (format: <output_dir>:<index>)
		parts := strings.Split(*testExtractFlag, ":")
		if len(parts) != 2 {
			fmt.Println("Error: Invalid format for -test-extract. Expected <output_dir>:<index>")
			fmt.Printf("Received: %s\n", *testExtractFlag)
			os.Exit(1)
		}

		outputDir := parts[0]
		indexStr := parts[1]

		fmt.Printf("TEST-EXTRACT: Extracting to directory '%s', index '%s'\n", outputDir, indexStr)

		// Convert the index string to an integer
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			fmt.Printf("Error: Invalid index '%s'. Must be a number.\n", indexStr)
			os.Exit(1)
		}

		// Extract the single file
		err = testExtractSingleFile(datFile, outputDir, index)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case *testFullCycleFlag != "":
		// Parse the test-full-cycle parameter (format: <temp_dir>:<index>)
		parts := strings.Split(*testFullCycleFlag, ":")
		if len(parts) != 2 {
			fmt.Println("Error: Invalid format for -test-full-cycle. Expected <temp_dir>:<index>")
			fmt.Printf("Received: %s\n", *testFullCycleFlag)
			os.Exit(1)
		}

		tempDir := parts[0]
		indexStr := parts[1]

		fmt.Printf("TEST-FULL-CYCLE: Using temp directory '%s', index '%s'\n", tempDir, indexStr)

		// Convert the index string to an integer
		index, err := strconv.Atoi(indexStr)
		if err != nil {
			fmt.Printf("Error: Invalid index '%s'. Must be a number.\n", indexStr)
			os.Exit(1)
		}

		// Run the full test cycle
		err = testPatchAndVerify(datFile, index, tempDir)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	default:
		fmt.Println("Error: You must specify one of -list, -extract, -update, -single-patch, or a test command")
		flag.Usage()
		os.Exit(1)
	}
}
