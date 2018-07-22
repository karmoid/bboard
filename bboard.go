package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// context : Store specific value to alter the program behaviour
// Like an Args container
type (
	context struct {
		src           *string
		verbose       *bool
		filecount     uint64
		fileprocessed uint64
		starttime     time.Time
		endtime       time.Time
	}
)

// contexte : Hold runtime value (from commande line args)
var contexte context

// Get the files' list to copy
func getFiles(ctx *context) (filesOut []os.FileInfo, errOut error) {
	pattern := filepath.Base(*ctx.src)
	files, err := ioutil.ReadDir(filepath.Dir(*ctx.src))
	if err != nil {
		log.Fatal(err)
	}
	for _, file := range files {
		if res, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(file.Name())); res {
			if err != nil {
				errOut = err
				return
			}
			filesOut = append(filesOut, file)
			// fmt.Printf("prise en compte de %s", file.Name())
		}
	}
	return filesOut, nil
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
	return nil
}

// No more Wildcard and selection in this Array
// fixedCopy because the Src array is predefined
func fixedCount(ctx *context, files []os.FileInfo) error {
	ctx.filecount = uint64(len(files))
	if *ctx.verbose {
		fmt.Printf("Files: %d\n",
			ctx.filecount)
		ctx.starttime = time.Now()
		fmt.Printf("**START** (%v)\n", ctx.starttime)
		defer func() { ctx.endtime = time.Now() }()
	}
	for _, file := range files {
		fmt.Printf("File processed : %s\n", file.Name())
		ctx.fileprocessed++
	}
	return nil
}

// Check if src is a wildcard expression
// if True, we must have a Path in dst
// Else dst could be Path or File
func genericCount(ctx *context) (myerr error) {
	// var files []os.FileInfo
	files, err := getFiles(ctx)
	if err != nil {
		return err
	}
	err = fixedCount(ctx, files)
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
	return err
}

// VersionNum : Litteral version
const VersionNum = "1.0"

func main() {
	fmt.Printf("bboard - Count files - C.m. 2018 - V%s\n", VersionNum)
	if err := processArgs(&contexte); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	err := genericCount(&contexte)
	if err != nil {
		fmt.Println("\nError:", err) // handle error
		os.Exit(1)
	}

	fmt.Println("\n Files:", err) // handle error
	os.Exit(0)
}
