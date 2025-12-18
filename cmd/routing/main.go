package main

import (
	"flag"
	"fmt"
	"os"
)

// Supported subcommands:
// - download: Download OSM PBF data
// - convert:  Convert to CH format
// - prepare:  Download + convert in one step
// - validate: Validate data integrity

func main() {
	// Subcommand definitions
	downloadCmd := flag.NewFlagSet("download", flag.ExitOnError)
	convertCmd := flag.NewFlagSet("convert", flag.ExitOnError)
	prepareCmd := flag.NewFlagSet("prepare", flag.ExitOnError)
	validateCmd := flag.NewFlagSet("validate", flag.ExitOnError)

	// download parameters
	downloadRegion := downloadCmd.String("region", "taiwan", "Region to download (taiwan, japan, etc.)")
	downloadOutput := downloadCmd.String("output", "/tmp", "Output directory for PBF file")

	// convert parameters
	convertInput := convertCmd.String("input", "", "Input PBF file path")
	convertOutput := convertCmd.String("output", "./data/routing", "Output directory for CSV files")
	convertContract := convertCmd.Bool("contract", true, "Enable CH contraction preprocessing")

	// prepare parameters (combines download + convert)
	prepareRegion := prepareCmd.String("region", "taiwan", "Region to download")
	prepareOutput := prepareCmd.String("output", "./data/routing", "Output directory for CSV files")

	// validate parameters
	validateDir := validateCmd.String("dir", "./data/routing", "Directory to validate")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "download":
		downloadCmd.Parse(os.Args[2:])
		runDownload(*downloadRegion, *downloadOutput)
	case "convert":
		convertCmd.Parse(os.Args[2:])
		runConvert(*convertInput, *convertOutput, *convertContract)
	case "prepare":
		prepareCmd.Parse(os.Args[2:])
		runPrepare(*prepareRegion, *prepareOutput)
	case "validate":
		validateCmd.Parse(os.Args[2:])
		runValidate(*validateDir)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: routing-cli <command> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  download    Download OSM PBF data")
	fmt.Println("  convert     Convert OSM PBF to CH format")
	fmt.Println("  prepare     Download and convert in one step")
	fmt.Println("  validate    Validate data integrity")
	fmt.Println("")
	fmt.Println("Use 'routing-cli <command> -h' for more information about a command.")
}

// Command implementations are in their respective files
