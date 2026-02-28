package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/charmbracelet/huh"
	"github.com/heysarver/devinfra/internal/compose"
	"github.com/heysarver/devinfra/internal/config"
	"github.com/heysarver/devinfra/internal/ui"
	"github.com/spf13/cobra"
)

var (
	flagImportFrom   string
	flagImportCA     string
	flagSkipPlatform bool
	flagTLD          string
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
	initCmd.Flags().StringVar(&flagImportCA, "import-ca", "", "path to a mkcert CAROOT directory from another machine to trust")
	initCmd.Flags().BoolVar(&flagSkipPlatform, "skip-platform", false, "skip platform-specific setup (requires sudo)")
	initCmd.Flags().StringVar(&flagTLD, "tld", "test", "local TLD to use for all projects (default: test)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Validate --import-ca path early, before doing anything else
	if flagImportCA != "" {
		if err := validateImportCAPath(flagImportCA); err != nil {
			return err
		}
	}

	// Check incompatible flag combination: --import-ca + --skip-platform on a fresh install
	if flagSkipPlatform && flagImportCA != "" && !config.IsInitialized() {
		return fmt.Errorf("--import-ca requires mkcert to be installed; remove --skip-platform or install mkcert manually first")
	}

	// Already initialized? Allow --import-ca to pass through; block everything else.
	if config.IsInitialized() {
		if flagImportCA != "" {
			return runImportCA(flagImportCA)
		}
		if flagTLD != "test" {
			return fmt.Errorf("devinfra is already initialized; to change the TLD manually edit %s, re-run platform setup, and regenerate infra certs", config.EnvFilePath())
		}
		if flagImportFrom == "" {
			ui.Ok("Already initialized at %s", config.ConfigDir())
			return nil
		}
	}

	// Collect TLD (flag or interactive wizard)
	tld, err := collectTLD()
	if err != nil {
		return err
	}
	if tld == "" {
		ui.Info("Cancelled.")
		return nil
	}

	// Ask about import when no --import-from flag and not skipping interactive mode
	if flagImportFrom == "" && !flagSkipPlatform && !flagYes {
		var wantsImport bool
		confirm := huh.NewConfirm().
			Title("Import an existing devinfra config?").
			Affirmative("Yes, import").
			Negative("No, start fresh").
			Value(&wantsImport)
		if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				ui.Info("Cancelled.")
				return nil
			}
			return err
		}
		if wantsImport {
			pathInput := huh.NewInput().
				Title("Path to existing config directory").
				Validate(func(s string) error {
					if _, err := os.Stat(s); err != nil {
						return fmt.Errorf("path not found: %s", s)
					}
					return nil
				}).
				Value(&flagImportFrom)
			if err := huh.NewForm(huh.NewGroup(pathInput)).Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					ui.Info("Cancelled.")
					return nil
				}
				return err
			}
		}
	}

	// Create directories
	ui.Info("Creating config directory at %s...", config.ConfigDir())
	if err := config.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Extract embedded compose files (rendered with chosen TLD)
	ui.Info("Extracting embedded resources...")
	if err := compose.ExtractEmbedded(tld); err != nil {
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
			scriptPath, err := compose.ExtractSetupScript(platform, tld)
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

	// Import and trust an external mkcert CA (runs after platform setup so mkcert is available)
	if flagImportCA != "" {
		if err := runImportCA(flagImportCA); err != nil {
			return err
		}
	}

	// Create Docker network
	ui.Info("Creating Docker network...")
	_ = compose.CreateNetwork(ctx)

	// Generate infrastructure certs
	ui.Info("Generating infrastructure certificates...")
	if err := compose.GenerateInfraCerts(ctx); err != nil {
		ui.Warn("Could not generate infra certs: %v", err)
		ui.Warn("Ensure mkcert is installed and run 'di init' again.")
	}

	// Write default .env if it doesn't exist
	envPath := config.EnvFilePath()
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		_ = os.WriteFile(envPath, []byte(fmt.Sprintf("DNS_PORT=%s\nTLD=%s\n", config.DNSPort(), tld)), 0644)
	}

	ui.Ok("Initialization complete!")
	if tld != "test" {
		ui.Warn("TLD is set to '.%s'. Changing TLD after projects are created requires regenerating all project certs.", tld)
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Next steps:")
	fmt.Fprintln(os.Stderr, "  di up      # Start Traefik + DNSMasq")
	fmt.Fprintln(os.Stderr, "  di doctor  # Verify everything works")
	return nil
}

// collectTLD returns the TLD to use: from the --tld flag when non-interactive,
// or from an interactive wizard prompt when running interactively.
// Returns ("", nil) if the user cancelled.
func collectTLD() (string, error) {
	tld := flagTLD

	// Interactive prompt when not --yes mode
	if !flagYes {
		input := huh.NewInput().
			Title("Local TLD to use for all projects").
			Description("Default is 'test' (.test domains). Common alternatives: local, dev").
			Placeholder("test").
			Value(&tld).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("TLD cannot be empty")
				}
				return nil
			})
		if err := huh.NewForm(huh.NewGroup(input)).Run(); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return "", nil
			}
			return "", err
		}
	}

	if err := config.ValidateTLD(tld); err != nil {
		return "", err
	}
	return tld, nil
}


// validateImportCAPath checks that the given path contains rootCA.pem.
func validateImportCAPath(caroot string) error {
	rootCA := filepath.Join(caroot, "rootCA.pem")
	if _, err := os.Stat(rootCA); err != nil {
		return fmt.Errorf("--import-ca: no rootCA.pem found in %s (is this a mkcert CAROOT directory?)", caroot)
	}
	return nil
}

// runImportCA installs the CA at caroot into the system trust stores using mkcert.
func runImportCA(caroot string) error {
	ui.Info("Trusting CA from %s...", caroot)
	cmd := exec.Command("mkcert", "-install")
	cmd.Env = append(os.Environ(), fmt.Sprintf("CAROOT=%s", caroot))
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("importing CA from %s: %w", caroot, err)
	}
	ui.Ok("CA from %s is now trusted.", caroot)
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
			_ = os.WriteFile(dest, data, 0600)
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
			_ = os.WriteFile(dest, data, 0644)
		}
		ui.Ok("Imported dynamic configs")
	}

	return nil
}
