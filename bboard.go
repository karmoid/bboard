package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// context : Store specific value to alter the program behaviour
// Like an Args container
type (
	fichier struct {
		path      string
		filecount int
	}

	context struct {
		src           *string
		verbose       *bool
		filecount     uint64
		fileprocessed uint64
		allfilesout   []os.FileInfo
		dirfilesout   []fichier
		starttime     time.Time
		endtime       time.Time
	}
)

// contexte : Hold runtime value (from commande line args)
var contexte context

func (f fichier) Name() string {
	return f.path
}

func (f fichier) Count() int {
	return f.filecount
}

// Check if path contains Wildcard characters
func isWildcard(value string) bool {
	return strings.Contains(value, "*") || strings.Contains(value, "?")
}

// Get the files' list to copy
func getFiles(ctx *context, src string) error {
	pattern := filepath.Base(src)
	files, err := ioutil.ReadDir(filepath.Dir(src))
	if err != nil {
		return err
	}
	for _, file := range files {
		if res, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(file.Name())); res {
			if err != nil {
				return err
			}
			ctx.allfilesout = append(ctx.allfilesout, file)
			// fmt.Printf("prise en compte de %s", file.Name())
		}
	}
	return nil
}

// Get the files' list to copy
func getFilesInPath(ctx *context, base string, lookfor string) error {
	// fmt.Printf("Looking for directory [%s] in [%s]", lookfor, base)
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", base, err)
			return err
		}
		if info.IsDir() && info.Name() == lookfor {
			// fmt.Printf("finding a dir with %s: %s - %+v \n", lookfor, path, info.Name())
			files, err := ioutil.ReadDir(filepath.Dir(path + "\\" + info.Name()))
			if err != nil {
				return err
			}
			ctx.dirfilesout = append(ctx.dirfilesout, fichier{path: path, filecount: len(files)})
		}

		return nil
	})

	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", base, err)
	}
	return nil
}

// Prepare Command Line Args parsing
func setFlagList(ctx *context) {
	ctx.src = flag.String("src", "", "Source file specification")
	ctx.verbose = flag.Bool("verbose", false, "Verbose mode")
	flag.Parse()
}

// Check args and return error if anything is wrong
func processArgs(ctx *context) (err error) {
	required := []string{"src"}
	setFlagList(&contexte)

	seen := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	for _, req := range required {
		if !seen[req] {
			// or possibly use `log.Fatalf` instead of:
			return fmt.Errorf("missing required -%s argument/flag", req)
		}
	}
	ctx.allfilesout = make([]os.FileInfo, 0, 300)
	ctx.dirfilesout = make([]fichier, 0, 100)

	return nil
}

// No more Wildcard and selection in this Array
// fixedCopy because the Src array is predefined
func fixedCount(ctx *context) {
	ctx.filecount = uint64(len(ctx.allfilesout))
	if *ctx.verbose {
		fmt.Printf("Files: %d\n",
			ctx.filecount)
		ctx.starttime = time.Now()
		fmt.Printf("**START** (%v)\n", ctx.starttime)
		defer func() { ctx.endtime = time.Now() }()
	}
	for _, file := range ctx.allfilesout {
		fmt.Printf("File processed : %s\n", file.Name())
		ctx.fileprocessed++
	}
	for _, file := range ctx.dirfilesout {
		fmt.Printf("Directory processed : %s - %d files\n", file.Name(), file.Count())
		ctx.fileprocessed++
	}
	return
}

// Check if src is a wildcard expression
// if True, we must have a Path in dst
// Else dst could be Path or File
func genericCount(ctx *context) bool {
	var haserror bool
	specs := strings.Split(*ctx.src, ";")
	for i := 0; i < len(specs); i++ {
		// fmt.Printf("[%d/%d] %s\n", i+1, len(specs), specs[i])
		if isWildcard(specs[i]) {
			// fmt.Print("common process on Wildcard\n")
			if err := getFiles(ctx, specs[i]); err != nil {
				haserror = true
				fmt.Errorf("Process error:", err)
			}
		} else if strings.HasSuffix(specs[i], "\\") {
			// fmt.Print("specific process on Directory\n")
			paths := strings.Split(specs[i], "\\")
			// for j := 0; j < len(paths); j++ {
			// 	fmt.Printf("%d - path: %s\n", j, paths[j])
			// }
			if len(paths) > 1 {
				base := paths[0] + "\\"
				startat := len(paths) - 1
				lookfor := paths[startat-1]
				if startat > 1 {
					startat--
				}
				for j := 1; j < startat; j++ {
					base = base + paths[j] + "\\"
				}
				if err := getFilesInPath(ctx, base, lookfor); err != nil {
					haserror = true
					fmt.Errorf("Process error: Not enough args [%s]", specs[i])
				}
			} else {
				haserror = true
				fmt.Errorf("Process error")
			}

		} else {
			fmt.Print("faultback process on Wildcard\n")
			if err := getFiles(ctx, specs[i]); err != nil {
				haserror = true
				fmt.Errorf("Process error:", err)
			}
		}
	}

	fixedCount(ctx)
	if *ctx.verbose {
		elapsedtime := ctx.endtime.Sub(ctx.starttime)
		seconds := int64(elapsedtime.Seconds())
		if seconds == 0 {
			seconds = 1
		}
		fmt.Printf("**END** (%v)\n  REPORT:\n  - Elapsed time: %v\n  - Files: %d processed on %d\n",
			ctx.endtime,
			elapsedtime,
			ctx.fileprocessed,
			ctx.filecount)
	}
	return haserror
}

// VersionNum : Litteral version
const VersionNum = "1.0"

func main() {
	fmt.Printf("bboard - Count files - C.m. 2018 - V%s\n", VersionNum)
	if err := processArgs(&contexte); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	if !genericCount(&contexte) {
		fmt.Println("\nWITH PROCESS ERROR\n") // handle error
		os.Exit(1)
	}

	os.Exit(0)
}
