package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/pkg/errors"
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
	convertRegion := convertCmd.String("region", "unknown", "Region of the input data (taiwan, japan, etc.)")

	// prepare parameters (combines download + convert)
	prepareRegion := prepareCmd.String("region", "taiwan", "Region to download")
	prepareOutput := prepareCmd.String("output", "./data/routing", "Output directory for CSV files")

	// validate parameters
	validateDir := validateCmd.String("dir", "./data/routing", "Directory to validate")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flags := routingFlags{
		Download: downloadFlags{
			cmd:    downloadCmd,
			region: downloadRegion,
			output: downloadOutput,
		},
		Convert: convertFlags{
			cmd:      convertCmd,
			input:    convertInput,
			output:   convertOutput,
			contract: convertContract,
			region:   convertRegion,
		},
		Prepare: prepareFlags{
			cmd:    prepareCmd,
			region: prepareRegion,
			output: prepareOutput,
		},
		Validate: validateFlags{
			cmd: validateCmd,
			dir: validateDir,
		},
	}

	if err := runSubcommand(ctx, &flags); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type routingFlags struct {
	Download downloadFlags
	Convert  convertFlags
	Prepare  prepareFlags
	Validate validateFlags
}

type downloadFlags struct {
	cmd    *flag.FlagSet
	region *string
	output *string
}

type convertFlags struct {
	cmd      *flag.FlagSet
	input    *string
	output   *string
	contract *bool
	region   *string
}

type prepareFlags struct {
	cmd    *flag.FlagSet
	region *string
	output *string
}

type validateFlags struct {
	cmd *flag.FlagSet
	dir *string
}

func runSubcommand(ctx context.Context, flags *routingFlags) error {
	switch os.Args[1] {
	case "download":
		return handleDownload(ctx, flags)
	case "convert":
		return handleConvert(ctx, flags)
	case "prepare":
		return handlePrepare(ctx, flags)
	case "validate":
		return handleValidate(flags)
	default:
		printUsage()

		return errors.New("unknown subcommand")
	}
}

func handleDownload(ctx context.Context, flags *routingFlags) error {
	if err := flags.Download.cmd.Parse(os.Args[2:]); err != nil {
		return errors.Wrap(err, "failed to parse download flags")
	}

	return runDownload(ctx, *flags.Download.region, *flags.Download.output)
}

func handleConvert(ctx context.Context, flags *routingFlags) error {
	if err := flags.Convert.cmd.Parse(os.Args[2:]); err != nil {
		return errors.Wrap(err, "failed to parse convert flags")
	}

	if *flags.Convert.input == "" {
		return errors.New("--input flag is required for convert command")
	}

	return runConvert(ctx, *flags.Convert.input, *flags.Convert.output, *flags.Convert.region, *flags.Convert.contract)
}

func handlePrepare(ctx context.Context, flags *routingFlags) error {
	if err := flags.Prepare.cmd.Parse(os.Args[2:]); err != nil {
		return errors.Wrap(err, "failed to parse prepare flags")
	}

	return runPrepare(ctx, *flags.Prepare.region, *flags.Prepare.output)
}

func handleValidate(flags *routingFlags) error {
	if err := flags.Validate.cmd.Parse(os.Args[2:]); err != nil {
		return errors.Wrap(err, "failed to parse validate flags")
	}

	return runValidate(*flags.Validate.dir)
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
