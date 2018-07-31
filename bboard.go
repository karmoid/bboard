package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
)

type detail int

const max_history = 10

const (
	Shortest = iota
	Longest
	Oldest
	Youngest
)

var byname = map[string]int{
	"shortest": Shortest,
	"longest":  Longest,
	"oldest":   Oldest,
	"youngest": Youngest,
}

// context : Store specific value to alter the program behaviour
// Like an Args container
type (
	// fichier struct {
	// 	path      string
	// 	filecount int
	// }
	StatAPI struct {
		Count     int           `json:"Count"`
		LessBytes int64         `json:"Lessbytes"`
		LbFile    string        `json:"LessbytesFile"`
		MoreBytes int64         `json:"Morebytes"`
		MbFile    string        `json:"MorebytesFile"`
		LessSecs  time.Duration `json:"Lesssecs"`
		LsFile    string        `json:"LesssecsFile"`
		MoreSecs  time.Duration `json:"Moresecs"`
		MsFile    string        `json:"MoresecsFile"`
	}

	DirectoryAPI struct {
		Path      string    `json:"Path"`
		Current   StatAPI   `json:"Current"`
		Histories []StatAPI `json:"Histories"`
	}

	DirectoriesAPI struct {
		Src         string         `json:"Src"`
		Directories []DirectoryAPI `json:"Directories"`
	}

	Stat struct {
		Count     int
		LessBytes int64
		MoreBytes int64
		LessSecs  time.Duration
		MoreSecs  time.Duration
		LbFile    string
		MbFile    string
		LsFile    string
		MsFile    string
	}

	Directory struct {
		Path      string
		Current   Stat
		Histories []Stat
	}

	Directories struct {
		Src         string
		Directories []Directory
	}

	context struct {
		src     *string
		verbose *bool
		quick   *string
		// details       *string
		flagNoColor   *bool
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

func (s Stat) dumpDetails() {
	if s.Count > 0 {
		fmt.Printf("\tOldest:(%s-%s)\n\tNewest:(%s-%s)\n\tSmallest:(%s-%s)\n\tLargest:(%s-%s)\n",
			s.LsFile, humanizeMinutes(int(s.LessSecs.Minutes())), s.MsFile, humanizeMinutes(int(s.MoreSecs.Minutes())), s.LbFile, humanize.Bytes(uint64(s.LessBytes)), s.MbFile, humanize.Bytes(uint64(s.MoreBytes)))

	}
}

func humanizeUnit(value int, base int, singular string) string {
	if value > base {
		days := value / base
		unit := ""
		if days > 1 {
			unit = "s"
		}
		return fmt.Sprintf("%d %s%s ", days, singular, unit)
	} else {
		return ""
	}
}

func humanizeMinutes(min int) string {
	var humanst string = ""
	humanst = humanst + humanizeUnit(min, 1440, "day")
	min = min % 1440
	humanst = humanst + humanizeUnit(min, 60, "hour")
	min = min % 60
	humanst = humanst + humanizeUnit(min, 1, "minute")
	humanst = strings.Trim(humanst, " ")
	if humanst != "" {
		return humanst
	}
	return "less than a minute"
}

func (s Stat) registerFile(file os.FileInfo) Stat {
	if !file.IsDir() {
		s.Count++
		delay := time.Since(file.ModTime())
		// elapsedtime := humanizeMinutes(int(delay.Minutes()))
		// if *contexte.verbose {
		// 	fmt.Printf("file %s, %v-(%s), %s\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
		// }
		if file.Size() > s.MoreBytes {
			s.MoreBytes = file.Size()
			s.MbFile = file.Name()
			// if *contexte.verbose {
			// 	fmt.Printf("Taking MoreBytes %s, %v-(%s), %d\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
		if file.Size() < s.LessBytes {
			s.LessBytes = file.Size()
			s.LbFile = file.Name()
			// if *contexte.verbose {
			// 	fmt.Printf("Taking LessBytes %s, %v-(%s), %d\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
		if delay > s.MoreSecs {
			s.MoreSecs = delay
			s.MsFile = file.Name()
			// if *contexte.verbose {
			// 	fmt.Printf("Taking MoreSecs %s, %v-(%s), %d\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
		if delay < s.LessSecs {
			s.LessSecs = delay
			s.LsFile = file.Name()
			// if *contexte.verbose {
			// 	fmt.Printf("Taking LessSecs %s, %v-(%s), %d\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
	}
	return s
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
	// if *ctx.verbose {
	// 	fmt.Printf("Looking for directory [%s] in [%s]", lookfor, base)
	//
	// }
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", base, err)
			return err
		}
		if info.IsDir() && info.Name() == lookfor {
			// if *ctx.verbose {
			// 	fmt.Printf("finding a dir with lookfor %s: path:%s - info.name():%+v \n", lookfor, path, info.Name())
			// 	fmt.Printf("Slow list :%s\n", path)
			// }
			files, err := ioutil.ReadDir(path)
			if err != nil {
				return err
			}
			curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}
			for _, file := range files {
				curr = curr.registerFile(file)
			}
			ctx.dirfilesout.Directories = append(ctx.dirfilesout.Directories, Directory{Path: path,
				Histories: make([]Stat, 0, 10),
				Current:   curr,
			})
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
	// ctx.details = flag.String("details", "", "File to store detail data - csv/xls mode")
	ctx.flagNoColor = flag.Bool("no-color", false, "Disable color output")
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
	if *ctx.flagNoColor {
		color.NoColor = true // disables colorized output
	}

	return nil
}

func analyzeHist(hist []Stat) (retour string) {
	if len(hist) > 1 {
		retour = "past"
		for i := len(hist) - 1; i > 0; i-- {
			retour = retour + fmt.Sprintf(":%+d", hist[i].Count-hist[i-1].Count)
		}
	} else {
		retour = ""
	}
	return
}

func getTrend(ctx *context, count int, hist []Stat) string {
	if ctx.processlist && len(hist) > 0 {
		return fmt.Sprintf(" (%+d)%s", count-hist[len(hist)-1].Count, analyzeHist(hist))
	}
	return ""
}

// No more Wildcard and selection in this Array
// fixedCopy because the Src array is predefined
func fixedCount(ctx *context) {
	ctx.filecount = uint64(len(ctx.allfilesout))
	if *ctx.verbose {
		fmt.Printf("Files: %d\n",
			ctx.filecount)
		fmt.Printf("**START** (%v)\n", ctx.starttime)
	}
	for _, file := range ctx.allfilesout {
		fmt.Printf("File processed : %s\n", file.Name())
		ctx.fileprocessed++
	}

	for _, file := range ctx.dirfilesout.Directories {
		color.Set(color.FgHiWhite)
		fmt.Printf("Directory processed : %s - %d files%s\n", file.Path, file.Current.Count, getTrend(ctx, file.Current.Count, file.Histories))
		color.Unset()
		if *ctx.verbose {
			file.Current.dumpDetails()
		}
		ctx.fileprocessed++
	}
	ctx.endtime = time.Now()
	if *ctx.verbose {
		elapsedtime := ctx.endtime.Sub(ctx.starttime)
		seconds := int64(elapsedtime.Seconds())
		if seconds == 0 {
			seconds = 1
		}
		fmt.Printf("**END** (%v)\n  REPORT:\n  - Elapsed time: %v\n  - Files/Dirs: %d processed\n",
			ctx.endtime,
			elapsedtime,
			ctx.fileprocessed,
		)
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
		files, err := ioutil.ReadDir(dir.Path)
		haserror = err != nil
		if len(ctx.dirfilesout.Directories[i].Histories) >= max_history {
			// fmt.Printf("Should manage fixed size (%d/%d)", len(ctx.dirfilesout.Directories[i].Histories), max_history)
			// for i, hist := range ctx.dirfilesout.Directories[i].Histories {
			// 	fmt.Printf("before trunc slice %d: Count:%d\n", i, hist.Count)
			// }
			neededHistories := ctx.dirfilesout.Directories[i].Histories[1:]
			// for i, hist := range neededHistories {
			// 	fmt.Printf("after trunc slice %d: Count:%d\n", i, hist.Count)
			// }
			copiedHistories := make([]Stat, max_history-1)
			copy(copiedHistories, neededHistories)
			// for i, hist := range copiedHistories {
			// 	fmt.Printf("after copied slice %d: Count:%d\n", i, hist.Count)
			// }
			ctx.dirfilesout.Directories[i].Histories = copiedHistories
		}
		ctx.dirfilesout.Directories[i].Histories = append(ctx.dirfilesout.Directories[i].Histories, dir.Current)
		// for i, hist := range ctx.dirfilesout.Directories[i].Histories {
		// 	fmt.Printf("after append %d: Count:%d\n", i, hist.Count)
		// }
		curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}
		for _, file := range files {
			curr = curr.registerFile(file)
			// fmt.Printf("%d, %s\n", curr.Count, curr.LbFile)
		}
		ctx.dirfilesout.Directories[i].Current = curr
	}

	fixedCount(ctx)
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
			if *ctx.verbose {
				// fmt.Print("specific process on Directory\n")
			}
			paths := strings.Split(specs[i], "\\")
			// if *ctx.verbose {
			// 	for j := 0; j < len(paths); j++ {
			// 		fmt.Printf("%d - path: %s\n", j, paths[j])
			// 	}
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
			// fmt.Print("faultback process on Wildcard\n")
			if err := getFiles(ctx, specs[i]); err != nil {
				haserror = true
				fmt.Errorf("Process error:", err)
			}
		}
	}

	fixedCount(ctx)
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

	contexte.starttime = time.Now()

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
		// fmt.Println("new json string: [", string(dirsJson), "]")
		// fmt.Printf("%+v", dirs)
		ioutil.WriteFile(*contexte.quick, dirsJson, 0644)
	}

	os.Exit(0)
}
