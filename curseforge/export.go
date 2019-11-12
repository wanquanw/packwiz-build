package curseforge

import (
	"archive/zip"
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strconv"

	"github.com/comp500/packwiz/curseforge/packinterop"

	"github.com/comp500/packwiz/core"
	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export the current modpack into a .zip for curseforge",
	// TODO: arguments for file name, author? projectID?
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Loading modpack...")
		pack, err := core.LoadPack()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		index, err := pack.LoadIndex()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// TODO: should index just expose indexPath itself, through a function?
		indexPath := filepath.Join(filepath.Dir(viper.GetString("pack-file")), filepath.FromSlash(pack.Index.File))

		// TODO: filter mods for optional/server/etc
		mods := loadMods(index)

		expFile, err := os.Create("export.zip")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		defer expFile.Close()
		exp := zip.NewWriter(expFile)
		defer exp.Close()

		cfFileRefs := make([]packinterop.AddonFileReference, 0, len(mods))
		for _, mod := range mods {
			projectRaw, ok := mod.GetParsedUpdateData("curseforge")
			// If the mod has curseforge metadata, add it to cfFileRefs
			// TODO: how to handle files with CF metadata, but with different download path?
			if ok {
				p := projectRaw.(cfUpdateData)
				cfFileRefs = append(cfFileRefs, packinterop.AddonFileReference{ProjectID: p.ProjectID, FileID: p.FileID})
			} else {
				// If the mod doesn't have the metadata, save it into the zip
				path, err := filepath.Rel(filepath.Dir(indexPath), mod.GetDestFilePath())
				if err != nil {
					fmt.Printf("Error resolving mod file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				modFile, err := exp.Create(filepath.ToSlash(filepath.Join("overrides", path)))
				if err != nil {
					fmt.Printf("Error creating mod file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				err = mod.DownloadFile(modFile)
				if err != nil {
					fmt.Printf("Error downloading mod file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
			}
		}

		manifestFile, err := exp.Create("manifest.json")
		if err != nil {
			fmt.Println("Error creating manifest: " + err.Error())
			os.Exit(1)
		}

		err = packinterop.WriteManifestFromPack(pack, cfFileRefs, manifestFile)
		if err != nil {
			fmt.Println("Error creating manifest: " + err.Error())
			os.Exit(1)
		}

		err = createModlist(exp, mods)
		if err != nil {
			fmt.Println("Error creating mod list: " + err.Error())
			os.Exit(1)
		}

		i := 0
		for _, v := range index.Files {
			if !v.MetaFile {
				// Save all non-metadata files into the zip
				path, err := filepath.Rel(filepath.Dir(indexPath), index.GetFilePath(v))
				if err != nil {
					fmt.Printf("Error resolving file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				file, err := exp.Create(filepath.ToSlash(filepath.Join("overrides", path)))
				if err != nil {
					fmt.Printf("Error creating file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				err = index.SaveFile(v, file)
				if err != nil {
					fmt.Printf("Error copying file: %s\n", err.Error())
					// TODO: exit(1)?
					continue
				}
				i++
			}
		}

		fmt.Println("Modpack exported to export.zip!")
	},
}

func createModlist(zw *zip.Writer, mods []core.Mod) error {
	modlistFile, err := zw.Create("modlist.html")
	if err != nil {
		return err
	}

	w := bufio.NewWriter(modlistFile)

	_, err = w.WriteString("<ul>\r\n")
	if err != nil {
		return err
	}
	for _, mod := range mods {
		projectRaw, ok := mod.GetParsedUpdateData("curseforge")
		if !ok {
			// TODO: read homepage URL or something similar?
			_, err = w.WriteString("<li>" + mod.Name + "</li>\r\n")
			if err != nil {
				return err
			}
			continue
		}
		project := projectRaw.(cfUpdateData)
		projIDString := strconv.Itoa(project.ProjectID)
		_, err = w.WriteString("<li><a href=\"https://minecraft.curseforge.com/mc-mods/" + projIDString + "\">" + mod.Name + "</a></li>\r\n")
		if err != nil {
			return err
		}
	}
	_, err = w.WriteString("</ul>\r\n")
	if err != nil {
		return err
	}
	return w.Flush()
}

func loadMods(index core.Index) []core.Mod {
	modPaths := index.GetAllMods()
	mods := make([]core.Mod, len(modPaths))
	i := 0
	fmt.Println("Reading mod files...")
	for _, v := range modPaths {
		modData, err := core.LoadMod(v)
		if err != nil {
			fmt.Printf("Error reading mod file: %s\n", err.Error())
			// TODO: exit(1)?
			continue
		}

		mods[i] = modData
		i++
	}
	return mods[:i]
}

func init() {
	curseforgeCmd.AddCommand(exportCmd)
}