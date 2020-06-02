package cmd

import (
	"fmt"
	"os"

	"github.com/anchore/imgbom/imgbom"
	"github.com/anchore/imgbom/imgbom/distro"
	_distro "github.com/anchore/imgbom/imgbom/distro"
	"github.com/anchore/imgbom/imgbom/scope"
	"github.com/anchore/stereoscope"
	"github.com/anchore/vulnscan/internal"
	"github.com/anchore/vulnscan/internal/db"
	"github.com/anchore/vulnscan/internal/format"
	"github.com/anchore/vulnscan/vulnscan"
	"github.com/anchore/vulnscan/vulnscan/vulnerability"
	hashiVer "github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   fmt.Sprintf("%s [IMAGE]", internal.ApplicationName),
	Short: "A container image vulnerability scanner", // TODO: add copy
	Long: format.Tprintf(`Supports the following image sources:
    {{.appName}} yourrepo/yourimage:tag             defaults to using images from a docker daemon
    {{.appName}} docker://yourrepo/yourimage:tag    explicitly use a docker daemon
    {{.appName}} tar://path/to/yourimage.tar        use a tarball from disk
`, map[string]interface{}{
		"appName": internal.ApplicationName,
	}),
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(runDefaultCmd(cmd, args))
	},
}

func init() {
	// setup CLI options specific to scanning an image

	// scan options
	flag := "scope"
	rootCmd.Flags().StringP(
		"scope", "s", scope.AllLayersScope.String(),
		fmt.Sprintf("selection of layers to analyze, options=%v", scope.Options))
	if err := viper.BindPFlag(flag, rootCmd.Flags().Lookup(flag)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", flag, err)
		os.Exit(1)
	}

	// output & formatting options
	flag = "output"
	rootCmd.Flags().StringP(
		// TODO: default option
		flag, "o", "text",
		// TODO: show all options
		fmt.Sprintf("report output formatter, options=%v", []string{}),
	)
	if err := viper.BindPFlag(flag, rootCmd.Flags().Lookup(flag)); err != nil {
		fmt.Printf("unable to bind flag '%s': %+v", flag, err)
		os.Exit(1)
	}
}

func runDefaultCmd(_ *cobra.Command, args []string) int {
	userImageStr := args[0]
	log.Infof("Fetching image '%s'", userImageStr)
	img, err := stereoscope.GetImage(userImageStr)
	if err != nil {
		log.Errorf("could not fetch image '%s': %w", userImageStr, err)
		return 1
	}
	defer stereoscope.Cleanup()

	log.Info("Cataloging image")
	catalog, err := imgbom.CatalogImage(img, appConfig.ScopeOpt)
	if err != nil {
		log.Errorf("could not catalog image: %w", err)
		return 1
	}

	// TODO: remove me (replace with imgbom os.Identify call)

	ver, err := hashiVer.NewVersion("8")
	if err != nil {
		panic(err)
	}

	osObj := _distro.Distro{
		Type:    distro.Debian,
		Version: ver,
	}

	// // TODO: remove me
	// // add vulnerable package
	// catalog := pkg.NewCatalog()
	// catalog.Add(pkg.Package{
	// 	Name:    "util-linux",
	// 	Version: "2.24.1-3",
	// 	Type:    pkg.DebPkg,
	// })

	// store := db.NewMockDb()
	store := db.GetStoreFromSqlite()
	provider := vulnerability.NewProviderFromStore(store)

	results := vulnscan.FindAllVulnerabilities(provider, osObj, catalog)

	count := 0
	for match := range results.Enumerate() {
		fmt.Println(match)
		count++
	}
	fmt.Printf("Found %d Vulnerabilities\n", count)

	return 0
}
