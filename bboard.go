package main

import (
	"encoding/json"
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
	// fichier struct {
	// 	path      string
	// 	filecount int
	// }

	HistoryAPI struct {
		Count int `json:"Count"`
	}

	DirectoryAPI struct {
		Path      string       `json:"Path"`
		Count     int          `json:"Count"`
		Histories []HistoryAPI `json:"Histories"`
	}

	DirectoriesAPI struct {
		Src         string         `json:"Src"`
		Directories []DirectoryAPI `json:"Directories"`
	}

	History struct {
		Count int
	}

	Directory struct {
		Path      string
		Count     int
		Histories []History
	}

	Directories struct {
		Src         string
		Directories []Directory
	}

	context struct {
		src           *string
		verbose       *bool
		quick         *string
		filecount     uint64
		fileprocessed uint64
		allfilesout   []os.FileInfo
		dirfilesout   Directories
		starttime     time.Time
		endtime       time.Time
		processlist   bool
	}
)

// contexte : Hold runtime value (from commande line args)
var contexte context

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
			ctx.dirfilesout.Directories = append(ctx.dirfilesout.Directories, Directory{Path: path, Count: len(files), Histories: make([]History, 0)})
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
	ctx.quick = flag.String("quickrefresh", "", "File to store cached data - quicker search/trend mode")
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
	ctx.dirfilesout = Directories{Src: *ctx.src, Directories: make([]Directory, 0, 100)}

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
	for _, file := range ctx.dirfilesout.Directories {
		fmt.Printf("Directory processed : %s - %d files\n", file.Path, file.Count)
		ctx.fileprocessed++
	}
	return
}

// Check if src is a wildcard expression
// if True, we must have a Path in dst
// Else dst could be Path or File
func listCount(ctx *context) bool {
	var haserror bool
	for i, dir := range ctx.dirfilesout.Directories {
		// fmt.Printf("Quick list %d:%s\n", i, dir.Path)
		files, err := ioutil.ReadDir(filepath.Dir(dir.Path))
		haserror = err != nil
		ctx.dirfilesout.Directories[i].Histories = append(ctx.dirfilesout.Directories[i].Histories, History{Count: dir.Count})
		ctx.dirfilesout.Directories[i].Count = len(files)
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

func getConfig(ctx *context) bool {
	file, err := os.Open(*ctx.quick)
	if err != nil {
		// fmt.Println("error:", err)
		// configSt, _ := json.Marshal(&ctx.dirfilesout)
		// fmt.Println("config:", configSt)
		// ioutil.WriteFile(*ctx.quick, configSt, 0644)
		return false
	}
	defer file.Close()
	// fmt.Println("Nous allons décoder", file)
	decoder := json.NewDecoder(file)
	Dir := Directories{}
	err = decoder.Decode(&Dir)
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	if Dir.Src != *ctx.src {
		fmt.Println("***Start from empty file. Different Src args***")
		return false
	}
	for _, onedir := range Dir.Directories {
		ctx.dirfilesout.Directories = append(ctx.dirfilesout.Directories, onedir)
		// fmt.Printf("Just added %d: %s-%d\n", i, onedir.Path, onedir.Count)
	}
	return true
}

// VersionNum : Litteral version
const VersionNum = "1.0"

func main() {
	fmt.Printf("bboard - Count files - C.m. 2018 - V%s\n", VersionNum)
	if err := processArgs(&contexte); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	if *contexte.quick != "" {
		contexte.processlist = getConfig(&contexte)
	}
	if contexte.processlist {
		if !listCount(&contexte) {
			fmt.Println("\nWITH PROCESS ERROR\n") // handle error
			// os.Exit(1)
		}

	} else {
		if !genericCount(&contexte) {
			fmt.Println("\nWITH PROCESS ERROR\n") // handle error
			// os.Exit(1)
		}
	}

	// var jsonBlob = []byte(`
	//       {"Src":"c:\\tools\\packages\\",
	// 				"Directories":[
	// 				{"Path":"c:\\tools\\toto\\packages\\1", "Count":12, "History":[13,24,34,56]},
	// 				{"Path":"c:\\tools\\toto\\packages\\2", "Count":21, "History":[31,12,4,6]}
	// 				]}
	//   `)
	//
	// dirsAPI := DirectoriesAPI{}
	// err := json.Unmarshal(jsonBlob, &dirsAPI)
	// if err != nil {
	// 	fmt.Errorf("opening config file : %v", err)
	// }
	// fmt.Printf("Unmarshalled : %v", dirsAPI)
	// fmt.Printf("find json. SRC=%s\n", dirsAPI.Src)
	// for i, direc := range dirsAPI.Directories {
	// 	fmt.Printf("%d: Dir %s, %d files\n", i, direc.Path, direc.Count)
	// 	for j, hist := range direc.History {
	// 		fmt.Printf("History %d: %d\n", j, hist)
	// 	}
	// }
	if *contexte.quick != "" {
		dirs := Directories(contexte.dirfilesout)
		// fmt.Printf("find json. SRC=%s\n", dirs.Src)
		dirsJson, _ := json.Marshal(dirs)
		fmt.Println("new json string: [", string(dirsJson), "]")
		// fmt.Printf("%+v", dirs)
		ioutil.WriteFile(*contexte.quick, dirsJson, 0644)
	}

	os.Exit(0)
}
