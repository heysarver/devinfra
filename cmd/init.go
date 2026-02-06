package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagImportFrom   string
	flagSkipPlatform bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize devinfra configuration",
	Long:  "Create config directory, extract embedded resources, run platform setup, create Docker network, and generate infrastructure certificates.",
	GroupID: "infra",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&flagImportFrom, "import-from", "", "import projects.yaml, certs, and dynamic configs from existing dev-infra repo")
	initCmd.Flags().BoolVar(&flagSkipPlatform, "skip-platform", false, "skip platform-specific setup (requires sudo)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Check if already initialized
	if config.IsInitialized() && flagImportFrom == "" {
		ui.Ok("Already initialized at %s", config.ConfigDir())
		return nil
	}

	// Create directories
	ui.Info("Creating config directory at %s...", config.ConfigDir())
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Extract embedded compose files
	ui.Info("Extracting embedded resources...")
	if err := compose.ExtractEmbedded(); err != nil {
		return fmt.Errorf("extracting embedded files: %w", err)
	}

	// Import from existing repo if requested
	if flagImportFrom != "" {
		ui.Info("Importing from %s...", flagImportFrom)
		if err := importFrom(flagImportFrom); err != nil {
			return fmt.Errorf("importing: %w", err)
		}
	}

	// Platform setup
	if !flagSkipPlatform {
		platform := runtime.GOOS
		if platform != "darwin" && platform != "linux" {
			ui.Warn("Platform %s not supported for automatic setup. Use --skip-platform.", platform)
		} else {
			ui.Info("Running platform setup (%s)...", platform)
			scriptPath, err := compose.ExtractSetupScript(platform)
			if err != nil {
				return fmt.Errorf("extracting setup script: %w", err)
			}
			setupCmd := exec.CommandContext(ctx, "bash", scriptPath)
			setupCmd.Stdout = os.Stderr
			setupCmd.Stderr = os.Stderr
			setupCmd.Stdin = os.Stdin
			setupCmd.Env = append(os.Environ(), fmt.Sprintf("DNS_PORT=%s", config.DNSPort()))
			if err := setupCmd.Run(); err != nil {
				ui.Warn("Platform setup had issues: %v", err)
				ui.Warn("Run 'di doctor' to check what needs fixing.")
			}
		}
	}

	// Create Docker network
	ui.Info("Creating Docker network...")
	compose.CreateNetwork(ctx)

	// Generate infrastructure certs
	ui.Info("Generating infrastructure certificates...")
	if err := compose.GenerateInfraCerts(ctx); err != nil {
		ui.Warn("Could not generate infra certs: %v", err)
		ui.Warn("Ensure mkcert is installed and run 'di init' again.")
	}

	// Write default .env if it doesn't exist
	envPath := config.EnvFilePath()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		os.WriteFile(envPath, []byte(fmt.Sprintf("DNS_PORT=%s\n", config.DNSPort())), 0644)
	}

	ui.Ok("Initialization complete!")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Next steps:")
	fmt.Fprintln(os.Stderr, "  di up      # Start Traefik + DNSMasq")
	fmt.Fprintln(os.Stderr, "  di doctor  # Verify everything works")
	return nil
}

func importFrom(srcDir string) error {
	// Import projects.yaml
	srcRegistry := filepath.Join(srcDir, "projects.yaml")
	if _, err := os.Stat(srcRegistry); err == nil {
		data, err := os.ReadFile(srcRegistry)
		if err != nil {
			return fmt.Errorf("reading source registry: %w", err)
		}
		if err := os.WriteFile(config.RegistryPath(), data, 0644); err != nil {
			return fmt.Errorf("writing registry: %w", err)
		}
		ui.Ok("Imported projects.yaml")
	}

	// Import certs
	srcCerts := filepath.Join(srcDir, "certs")
	if entries, err := os.ReadDir(srcCerts); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(srcCerts, e.Name()))
			if err != nil {
				continue
			}
			dest := filepath.Join(config.CertsDir(), e.Name())
			os.WriteFile(dest, data, 0600)
		}
		ui.Ok("Imported certificates")
	}

	// Import dynamic configs
	srcDynamic := filepath.Join(srcDir, "dynamic")
	if entries, err := os.ReadDir(srcDynamic); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(srcDynamic, e.Name()))
			if err != nil {
				continue
			}
			dest := filepath.Join(config.DynamicDir(), e.Name())
			os.WriteFile(dest, data, 0644)
		}
		ui.Ok("Imported dynamic configs")
	}

	return nil
}
