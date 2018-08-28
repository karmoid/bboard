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
const p_debug = false

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
		Directories map[string]Directory
	}

	context struct {
		src     *string
		verbose *bool
		filter0 *bool
		quick   *string
		// details       *string
		flagNoColor   *bool
		readonly      *bool
		feedback      *int
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
			s.LsFile, humanizeMinutes(int(s.MoreSecs.Minutes())), s.MsFile, humanizeMinutes(int(s.LessSecs.Minutes())), s.LbFile, humanize.Bytes(uint64(s.LessBytes)), s.MbFile, humanize.Bytes(uint64(s.MoreBytes)))
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
		// if p_debug {
		// 	fmt.Printf("file %s, %v-(%s), %s\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
		// 	fmt.Printf("current stat %s, %v->%v, %s->%s\n", file.Name(), s.LessSecs, s.MoreSecs, humanize.Bytes(uint64(s.LessBytes)), humanize.Bytes(uint64(s.MoreBytes)))
		// }
		if file.Size() > s.MoreBytes {
			s.MoreBytes = file.Size()
			s.MbFile = file.Name()
			// if p_debug {
			// 	fmt.Printf("Taking MoreBytes %s, %v-(%s), %s\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
		if file.Size() < s.LessBytes {
			s.LessBytes = file.Size()
			s.LbFile = file.Name()
			// if p_debug {
			// 	fmt.Printf("Taking LessBytes %s, %v-(%s), %s\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
		if delay > s.MoreSecs {
			s.MoreSecs = delay
			s.MsFile = file.Name()
			// if p_debug {
			// 	fmt.Printf("Taking MoreSecs %s, %v-(%s), %s\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
			// }
		}
		if delay < s.LessSecs {
			s.LessSecs = delay
			s.LsFile = file.Name()
			// if p_debug {
			// 	fmt.Printf("Taking LessSecs %s, %v-(%s), %s\n", file.Name(), file.ModTime(), elapsedtime, humanize.Bytes(uint64(file.Size())))
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
	filecount := 0
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", base, err)
			return err
		}
		filecount++
		if *ctx.feedback > 0 && filecount%*ctx.feedback == 0 {
			fmt.Printf("(%d)-", filecount)
		}
		if info.IsDir() {
			if strings.ToLower(info.Name()) == strings.ToLower(lookfor) {
				// if p_debug {
				// 	fmt.Printf("INCLUDED: %s, %s, %v\n", path, info.Name(), info.IsDir())
				// }

				// if *ctx.verbose {
				// 	fmt.Printf("finding a dir with lookfor %s: path:%s - info.name():%+v \n", lookfor, path, info.Name())
				// 	fmt.Printf("Slow list :%s\n", path)
				// }

				// files, err := ioutil.ReadDir(path)
				// if err != nil {
				// 	if *ctx.details {
				// 		fmt.Printf("Exit for Walk with error : %v\n", err)
				// 	}
				// 	return err
				// }

				curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}

				// for _, file := range files {
				// 	curr = curr.registerFile(file)
				// }

				ctx.dirfilesout.Directories[path] = Directory{Path: path, Histories: make([]Stat, 0, 10), Current: curr}
			} else {
				// if p_debug {
				// 	fmt.Printf("--- Directory Excluded: %s, %s, %v\n", path, info.Name(), info.IsDir())
				// }
			}
		} else {
			// Not Dir. So File
			paths := strings.Split(path, "\\")
			// if p_debug {
			// 	for j := 0; j < len(paths); j++ {
			// 		fmt.Printf("%d - path: %s\n", j, paths[j])
			// 	}
			// }
			if strings.ToLower(paths[len(paths)-2]) == strings.ToLower(lookfor) {
				// if p_debug {
				// 	fmt.Printf("--- File Included: %s, %s, %v\n", path, info.Name(), info.IsDir())
				// }
				rootpath := strings.Join(paths[0:len(paths)-1], "\\")
				// if p_debug {
				// 	fmt.Printf("--- Path rebuilt : %s\n", rootpath)
				// }
				dir := ctx.dirfilesout.Directories[rootpath]
				dir.Current = dir.Current.registerFile(info)
				ctx.dirfilesout.Directories[rootpath] = dir
			} else {
				// if p_debug {
				// 	fmt.Printf("--- Excluded: %s, %s, %v\n", path, info.Name(), info.IsDir())
				// }
			}
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
	ctx.filter0 = flag.Bool("filternull", false, "Filtering 0 valued line")
	ctx.quick = flag.String("quickrefresh", "", "File to store cached data - quicker search/trend mode")
	// ctx.details = flag.String("details", "", "File to store detail data - csv/xls mode")
	ctx.readonly = flag.Bool("readonly", false, "don't get files. Dump json file")
	ctx.feedback = flag.Int("feedback", 0, "Display file processing (feedback count)")
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
	ctx.dirfilesout = Directories{Src: *ctx.src, Directories: map[string]Directory{}}
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

func getTrend(ctx *context, count int, hist []Stat) (bool, string) {
	if ctx.processlist && len(hist) > 0 {
		return (count > 0 || count-hist[len(hist)-1].Count != 0), fmt.Sprintf(" (%+d)%s", count-hist[len(hist)-1].Count, analyzeHist(hist))
	}
	return false, ""
}

// No more Wildcard and selection in this Array
// fixedCopy because the Src array is predefined
func fixedCount(ctx *context) {
	ctx.filecount = uint64(len(ctx.allfilesout))
	if *ctx.verbose {
		fmt.Printf("Files: %d\n", ctx.filecount)
		fmt.Printf("**START** (%v)\n", ctx.starttime)
	}
	for _, file := range ctx.allfilesout {
		fmt.Printf("File processed : %s\n", file.Name())
		ctx.fileprocessed++
	}

	for _, file := range ctx.dirfilesout.Directories {
		highlight, trend := getTrend(ctx, file.Current.Count, file.Histories)
		highlight = highlight || (*ctx.readonly && file.Current.Count > 0)

		if highlight {
			color.Set(color.FgHiWhite)
		}
		if !*ctx.filter0 || highlight {
			fmt.Printf("Directory processed : %s - %d files%s\n", file.Path, file.Current.Count, trend)
		}
		if highlight {
			color.Unset()
		}
		if *ctx.verbose {
			if !*ctx.filter0 || highlight {
				file.Current.dumpDetails()
			}
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

// if we had a Json file, and in a quickrefresh, we'll use the file entries
//
func listCount(ctx *context) bool {
	var haserror bool
	if *ctx.verbose {
		fmt.Println("Quick Process - %d Directories", len(ctx.dirfilesout.Directories))
	}
	filecount := 0
	if *ctx.readonly {
		if *ctx.verbose {
			fmt.Println("Read Quick list")
		}
	} else {
		for i, dir := range ctx.dirfilesout.Directories {
			if p_debug {
				fmt.Printf("Refresh Quick list %s %d\n", dir.Path, dir.Current.Count)
			}
			files, err := ioutil.ReadDir(dir.Path)
			haserror = err != nil
			if len(ctx.dirfilesout.Directories[i].Histories) >= max_history {
				// fmt.Printf("Should manage fixed size (%d/%d)", len(ctx.dirfilesout.Directories[i].Histories), max_history)
				// for i, hist := range ctx.dirfilesout.Directories[i].Histories {
				// 	fmt.Printf("before trunc slice %d: Count:%d\n", i, hist.Count)
				// }
				neededHistories := dir.Histories[1:]
				// for i, hist := range neededHistories {
				// 	fmt.Printf("after trunc slice %d: Count:%d\n", i, hist.Count)
				// }
				copiedHistories := make([]Stat, max_history-1)
				copy(copiedHistories, neededHistories)
				// for i, hist := range copiedHistories {
				// 	fmt.Printf("after copied slice %d: Count:%d\n", i, hist.Count)
				// }
				dir.Histories = copiedHistories
			}
			dir.Histories = append(dir.Histories, dir.Current)
			// for i, hist := range ctx.dirfilesout.Directories[i].Histories {
			// 	fmt.Printf("after append %d: Count:%d\n", i, hist.Count)
			// }
			curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}
			for _, file := range files {
				curr = curr.registerFile(file)
				filecount++
				if *ctx.feedback > 0 && filecount%*ctx.feedback == 0 {
					fmt.Printf("(%d)-", filecount)
				}
				// fmt.Printf("%d, %s\n", curr.Count, curr.LbFile)
			}
			dir.Current = curr
			ctx.dirfilesout.Directories[i] = dir
		}
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
		ctx.dirfilesout.Directories[onedir.Path] = onedir
		// fmt.Printf("Just added %d: %s-%d\n", i, onedir.Path, onedir.Count)
	}
	return true
}

// VersionNum : Litteral version
// 1.0 : Original
// 1.1 : Highlight important data
// 1.2 : Optimization on first discovery. Walk already work on files. So use Walk file entry
const VersionNum = "1.2"

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
