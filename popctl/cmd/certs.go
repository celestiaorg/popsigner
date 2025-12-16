package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Bidon15/popsigner/popctl/internal/api"
	"github.com/spf13/cobra"
)

var certsCmd = &cobra.Command{
	Use:   "certs",
	Short: "Manage mTLS certificates",
	Long: `Certificate management commands for Arbitrum Nitro mTLS integration.

Examples:
  popctl certs list
  popctl certs create nitro-production --validity 8760h
  popctl certs get 01HXYZ...
  popctl certs revoke 01HXYZ...
  popctl certs delete 01HXYZ...
  popctl certs download-ca`,
}

var certsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all certificates",
	RunE:  runCertsList,
}

var certsCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new mTLS certificate",
	Long: `Generate a new client certificate for Arbitrum Nitro mTLS authentication.

The certificate bundle will be saved to the current directory unless
--output-dir is specified.

Examples:
  popctl certs create nitro-production
  popctl certs create staker-cert --validity 4380h --output-dir ./certs`,
	Args: cobra.ExactArgs(1),
	RunE: runCertsCreate,
}

var certsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get certificate details",
	Args:  cobra.ExactArgs(1),
	RunE:  runCertsGet,
}

var certsRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke a certificate",
	Long: `Revoke an mTLS certificate. This action cannot be undone.

WARNING: Any services using this certificate will immediately lose access
to the POPSigner RPC gateway.`,
	Args: cobra.ExactArgs(1),
	RunE: runCertsRevoke,
}

var certsDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a revoked certificate",
	Long: `Permanently delete a revoked certificate record.

Only certificates that have been revoked can be deleted.`,
	Args: cobra.ExactArgs(1),
	RunE: runCertsDelete,
}

var certsDownloadCACmd = &cobra.Command{
	Use:   "download-ca",
	Short: "Download the CA certificate",
	Long: `Download the POPSigner CA certificate.

This CA certificate is needed to verify the server's identity when
connecting to the POPSigner RPC gateway.`,
	RunE: runCertsDownloadCA,
}

func init() {
	// List flags
	certsListCmd.Flags().String("status", "", "filter by status (active, expired, revoked)")

	// Create flags
	certsCreateCmd.Flags().String("validity", "8760h", "certificate validity period (e.g., 720h, 4380h, 8760h)")
	certsCreateCmd.Flags().StringP("output-dir", "o", ".", "directory to save certificate files")

	// Revoke flags
	certsRevokeCmd.Flags().String("reason", "", "reason for revocation")
	certsRevokeCmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")

	// Delete flags
	certsDeleteCmd.Flags().BoolP("force", "f", false, "skip confirmation prompt")

	// Download CA flags
	certsDownloadCACmd.Flags().StringP("output", "o", "popsigner-ca.crt", "output file name")

	// Add subcommands
	certsCmd.AddCommand(certsListCmd)
	certsCmd.AddCommand(certsCreateCmd)
	certsCmd.AddCommand(certsGetCmd)
	certsCmd.AddCommand(certsRevokeCmd)
	certsCmd.AddCommand(certsDeleteCmd)
	certsCmd.AddCommand(certsDownloadCACmd)

	rootCmd.AddCommand(certsCmd)
}

func runCertsList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	status, _ := cmd.Flags().GetString("status")

	certs, err := client.ListCertificates(ctx, status)
	if err != nil {
		printError(err)
		return err
	}

	if jsonOut {
		return printJSON(map[string]interface{}{
			"certificates": certs,
			"count":        len(certs),
		})
	}

	if len(certs) == 0 {
		fmt.Println("No certificates found")
		return nil
	}

	w := newTable()
	printTableHeader(w, "ID", "NAME", "STATUS", "FINGERPRINT", "EXPIRES")
	for _, c := range certs {
		status := formatCertStatus(c.Status)
		expiry := formatExpiry(c.ExpiresAt, c.Status)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			truncate(c.ID, 12),
			c.Name,
			status,
			truncate(c.Fingerprint, 16),
			expiry,
		)
	}
	return w.Flush()
}

func runCertsCreate(cmd *cobra.Command, args []string) error {
	certName := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}

	validity, _ := cmd.Flags().GetString("validity")
	outputDir, _ := cmd.Flags().GetString("output-dir")

	ctx := context.Background()

	fmt.Printf("Generating certificate '%s'...\n", certName)

	bundle, err := client.CreateCertificate(ctx, api.CreateCertificateRequest{
		Name:           certName,
		ValidityPeriod: validity,
	})
	if err != nil {
		printError(err)
		return err
	}

	if jsonOut {
		return printJSON(bundle)
	}

	// Save certificate files
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	certFile := filepath.Join(outputDir, "client.crt")
	keyFile := filepath.Join(outputDir, "client.key")
	caFile := filepath.Join(outputDir, "popsigner-ca.crt")

	if err := os.WriteFile(certFile, []byte(bundle.ClientCert), 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	if err := os.WriteFile(keyFile, []byte(bundle.ClientKey), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	if err := os.WriteFile(caFile, []byte(bundle.CACert), 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	fmt.Printf("%s Certificate generated successfully!\n\n", colorGreen("✓"))
	fmt.Printf("  Certificate: %s\n", certFile)
	fmt.Printf("  Private Key: %s\n", keyFile)
	fmt.Printf("  CA Cert:     %s\n", caFile)
	fmt.Printf("\n  Fingerprint: %s\n", truncate(bundle.Fingerprint, 32))
	fmt.Printf("  Expires:     %s\n", bundle.ExpiresAt)

	fmt.Printf("\n%s Nitro Configuration:\n\n", colorYellow("ℹ"))
	fmt.Println(bundle.NitroConfigTip)

	fmt.Printf("\n%s Store these files securely. The private key will not be shown again.\n", colorYellow("⚠"))

	return nil
}

func runCertsGet(cmd *cobra.Command, args []string) error {
	certID := args[0]

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	cert, err := client.GetCertificate(ctx, certID)
	if err != nil {
		printError(err)
		return err
	}

	if jsonOut {
		return printJSON(cert)
	}

	fmt.Printf("ID:           %s\n", cert.ID)
	fmt.Printf("Name:         %s\n", cert.Name)
	fmt.Printf("Status:       %s\n", formatCertStatus(cert.Status))
	fmt.Printf("CommonName:   %s\n", cert.CommonName)
	fmt.Printf("Serial:       %s\n", cert.SerialNumber)
	fmt.Printf("Fingerprint:  %s\n", cert.Fingerprint)
	fmt.Printf("Issued:       %s\n", cert.IssuedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Expires:      %s\n", cert.ExpiresAt.Format("2006-01-02 15:04:05"))
	if cert.RevokedAt != nil {
		fmt.Printf("Revoked:      %s\n", cert.RevokedAt.Format("2006-01-02 15:04:05"))
	}
	if cert.RevocationReason != nil && *cert.RevocationReason != "" {
		fmt.Printf("Revoke Reason: %s\n", *cert.RevocationReason)
	}
	fmt.Printf("Created:      %s\n", cert.CreatedAt.Format("2006-01-02 15:04:05"))

	return nil
}

func runCertsRevoke(cmd *cobra.Command, args []string) error {
	certID := args[0]

	force, _ := cmd.Flags().GetBool("force")
	reason, _ := cmd.Flags().GetString("reason")

	if !force {
		fmt.Printf("%s Are you sure you want to revoke certificate %s?\n", colorYellow("⚠"), certID)
		fmt.Printf("This action cannot be undone and any services using this certificate will lose access.\n")
		fmt.Print("[y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted")
			return nil
		}
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	if err := client.RevokeCertificate(ctx, certID, reason); err != nil {
		printError(err)
		return err
	}

	if jsonOut {
		return printJSON(map[string]string{
			"status":  "revoked",
			"message": fmt.Sprintf("Certificate %s revoked", certID),
		})
	}

	fmt.Printf("%s Certificate revoked: %s\n", colorGreen("✓"), certID)
	return nil
}

func runCertsDelete(cmd *cobra.Command, args []string) error {
	certID := args[0]

	force, _ := cmd.Flags().GetBool("force")

	if !force {
		fmt.Printf("%s Are you sure you want to delete certificate %s? [y/N]: ", colorYellow("⚠"), certID)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted")
			return nil
		}
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx := context.Background()

	if err := client.DeleteCertificate(ctx, certID); err != nil {
		printError(err)
		return err
	}

	if jsonOut {
		return printJSON(map[string]string{
			"status":  "deleted",
			"message": fmt.Sprintf("Certificate %s deleted", certID),
		})
	}

	fmt.Printf("%s Certificate deleted: %s\n", colorGreen("✓"), certID)
	return nil
}

func runCertsDownloadCA(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	output, _ := cmd.Flags().GetString("output")

	ctx := context.Background()

	caPEM, err := client.GetCACertificate(ctx)
	if err != nil {
		printError(err)
		return err
	}

	if jsonOut {
		return printJSON(map[string]string{
			"ca_cert": string(caPEM),
		})
	}

	if err := os.WriteFile(output, caPEM, 0644); err != nil {
		return fmt.Errorf("failed to write CA certificate: %w", err)
	}

	fmt.Printf("%s CA certificate saved to: %s\n", colorGreen("✓"), output)
	return nil
}

// Helper functions

func formatCertStatus(status api.CertificateStatus) string {
	switch status {
	case api.CertificateStatusActive:
		return colorGreen("ACTIVE")
	case api.CertificateStatusExpired:
		return colorYellow("EXPIRED")
	case api.CertificateStatusRevoked:
		return colorRed("REVOKED")
	default:
		return string(status)
	}
}

func formatExpiry(expiresAt time.Time, status api.CertificateStatus) string {
	if status == api.CertificateStatusRevoked {
		return "-"
	}

	expires := expiresAt.Format("Jan 2, 2006")

	// Check if expiring soon (within 30 days)
	if status == api.CertificateStatusActive && expiresAt.Before(time.Now().Add(30*24*time.Hour)) {
		expires = colorYellow(expires + " ⚠")
	}

	return expires
}

