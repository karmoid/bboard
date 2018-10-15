package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
		Base      string
		Path      string
		Current   Stat
		Histories []Stat
	}

	Directories struct {
		Src         string
		Directories map[string]Directory
	}

	context struct {
		src           *string
		verbose       *bool
		filter0       *bool
		quick         *string
		exclude       *string
		details       *string
		errors        *string
		influxdb      *string
		flagNoColor   *bool
		replay        *bool
		flagtree      *bool
		selectfile    *string
		feedback      *int
		history       *int
		filecount     uint64
		dircount      uint64
		fileprocessed uint64
		allfilesout   []os.FileInfo
		dirfilesout   Directories
		detailsout    *os.File
		errorsout     *os.File
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
		if file.Size() > s.MoreBytes {
			s.MoreBytes = file.Size()
			s.MbFile = file.Name()
		}
		if file.Size() < s.LessBytes {
			s.LessBytes = file.Size()
			s.LbFile = file.Name()
		}
		if delay > s.MoreSecs {
			s.MoreSecs = delay
			s.MsFile = file.Name()
		}
		if delay < s.LessSecs {
			s.LessSecs = delay
			s.LsFile = file.Name()
		}
	}
	return s
}

func (s Stat) registerDir(file os.FileInfo) Stat {
	if !file.IsDir() {
		s.Count++
		delay := time.Since(file.ModTime())
		s.MoreBytes = s.MoreBytes + file.Size()
		s.LessBytes = s.LessBytes + file.Size()
		if delay > s.MoreSecs {
			s.MoreSecs = delay
			s.MsFile = file.Name()
		}
		if delay < s.LessSecs {
			s.LessSecs = delay
			s.LsFile = file.Name()
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
			ctx.filecount++
			ctx.allfilesout = append(ctx.allfilesout, file)
			if *ctx.verbose {
				if len(ctx.allfilesout) > 300 {
					fmt.Printf("Append reach 300 limit and more : %d", len(ctx.allfilesout))
				}
			}
			// fmt.Printf("prise en compte de %s", file.Name())
		}
	}
	return nil
}

// Walk on Tree to calculate size and get oldest and youngest file
func walkontree(ctx *context, base string) (stat Stat) {
	stat = Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: int64(0), LessBytes: int64(0)}
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if *ctx.errors != "" {
				if _, err := io.WriteString(ctx.errorsout, fmt.Sprintf("prevent panic by handling failure accessing a path %q: %s - %v\n", base, path, err)); err != nil {
					fmt.Printf("unable to log error on %q: %s, %v\n", base, path, err)
					os.Exit(1)
				}
				return err
			}
			fmt.Printf("Error %q: %s, %v\n", base, path, err)
			return err
		}
		stat = stat.registerDir(info)
		return nil
	})

	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", base, err)
	}

	return stat
}

// Get the files' list to copy
func getFilesInPath(ctx *context, base string, lookingfor string) error {
	look := strings.Split(lookingfor, ";")
	exclude := strings.Split(*ctx.exclude, ";")
	// var filecount uint64 = 0
	// var dircount uint64 = 0
	couldprocess := false
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if *ctx.errors != "" {
				if _, err := io.WriteString(ctx.errorsout, fmt.Sprintf("prevent panic by handling failure accessing a path %q: %s - %v\n", base, path, err)); err != nil {
					fmt.Printf("unable to log error on %q: %s, %v\n", base, path, err)
					os.Exit(1)
				}
				return filepath.SkipDir
			}
			fmt.Printf("Error %q: %s, %v\n", base, path, err)
			return err
		}
		if uint64(*ctx.feedback) > 0 && ctx.filecount%uint64(*ctx.feedback) == 0 {
			fmt.Printf("f/d(%d/%d)\r", ctx.filecount, ctx.dircount)
		}
		if info.IsDir() {
			ctx.dircount++
			for i := 0; i < len(exclude); i++ {
				if strings.ToLower(info.Name()) == strings.ToLower(exclude[i]) {
					if *ctx.verbose {
						fmt.Printf("Skipped %s because in exclude list %s [%s]\n", path, *ctx.exclude, exclude[i])
					}
					return filepath.SkipDir
				}
			}
			couldprocess = false
			for i := 0; i < len(look); i++ {
				couldprocess = couldprocess || strings.ToLower(info.Name()) == strings.ToLower(look[i])
			}
			if couldprocess {
				// fmt.Print("path", path, "base", base)
				curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}
				ctx.dirfilesout.Directories[path] = Directory{Base: base, Path: path, Histories: make([]Stat, 0, 10), Current: curr}
			} else if *ctx.flagtree {
				paths := strings.Split(path, "\\")
				couldprocess = false
				for j := 0; j < len(look); j++ {
					couldprocess = couldprocess || strings.ToLower(paths[len(paths)-2]) == strings.ToLower(look[j])
				}
				if couldprocess {
					// fmt.Printf("On pourrait traiter le répertoire %s\n", path)
					ctx.dircount++
					curr := walkontree(ctx, path)
					// curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}
					ctx.dirfilesout.Directories[path] = Directory{Base: base, Path: path, Histories: make([]Stat, 0, 10), Current: curr}
					if *ctx.details != "" {
						if _, err := io.WriteString(ctx.detailsout, fmt.Sprintf("%s\t%s\t%d\t%d\t%s\t%d\t%s\t%d\t%s\n", base, path,
							curr.Count, curr.LessBytes, humanize.Bytes(uint64(curr.LessBytes)),
							int(curr.MoreSecs.Minutes()), humanizeMinutes(int(curr.MoreSecs.Minutes())),
							int(curr.LessSecs.Minutes()), humanizeMinutes(int(curr.LessSecs.Minutes())))); err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
					}
				}
			}
		} else {
			ctx.filecount++
			// Not Dir. So File
			paths := strings.Split(path, "\\")
			couldprocess = false
			for i := 0; i < len(look); i++ {
				couldprocess = couldprocess || strings.ToLower(paths[len(paths)-2]) == strings.ToLower(look[i])
			}
			if !*ctx.flagtree && couldprocess {
				rootpath := strings.Join(paths[0:len(paths)-1], "\\")
				if *ctx.details != "" {
					if _, err := io.WriteString(ctx.detailsout, fmt.Sprintf("%s\t%s\t%v\t%d\n", rootpath, info.Name(), info.ModTime(), info.Size())); err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
				}
				dir := ctx.dirfilesout.Directories[rootpath]
				dir.Current = dir.Current.registerFile(info)
				ctx.dirfilesout.Directories[rootpath] = dir
			}
		}
		return nil
	})
	fmt.Printf("Processed files(%d) & Directories(%d)\n", ctx.filecount, ctx.dircount)

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
	ctx.exclude = flag.String("exclude", "", "Directories to exclude")
	ctx.details = flag.String("details", "", "File to store detail data - xls format, tab separator")
	ctx.errors = flag.String("errors", "", "File to store errors list - txt format")
	ctx.replay = flag.Bool("replay", false, "don't get files. Replay from json file")
	ctx.selectfile = flag.String("select", "", "File/Dir select (contains)")
	ctx.feedback = flag.Int("feedback", 0, "Display file processing (feedback count)")
	ctx.history = flag.Int("history", max_history, "Keep historical data maximum")
	ctx.flagNoColor = flag.Bool("no-color", false, "Disable color output")
	ctx.flagtree = flag.Bool("tree", false, "Tree Size mode")
	ctx.influxdb = flag.String("influxdb", "", "Standard output for InfluxDB. Specify tablename.")
	flag.Parse()
}

// Check args and return error if anything is wrong
func processArgs(ctx *context) (err error) {
	setFlagList(ctx)
	if *ctx.src == "" {
		if !*ctx.replay {
			return fmt.Errorf("missing required -src argument/flag")
		}
	}

	if *ctx.flagNoColor {
		color.NoColor = true // disables colorized output
	}

	if *ctx.influxdb != "" {
		*ctx.verbose = false
	} else {
		fmt.Printf("bboard - Files analysis - C.m. 2018 - V%s\n", VersionNum)
	}

	if *ctx.flagtree {
		// *ctx.quick = ""
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
	// ctx.filecount = 0
	if *ctx.verbose {
		// fmt.Printf("Files: %d\n", ctx.filecount)
		fmt.Printf("**START** (%v)\n", ctx.starttime)
	}
	for _, file := range ctx.allfilesout {
		if *ctx.selectfile == "" || strings.Contains(strings.ToLower(file.Name()), strings.ToLower(*ctx.selectfile)) {
			fmt.Printf("File processed : %s\n", file.Name())
			ctx.fileprocessed++
		}
		// ctx.filecount++
	}
	highlighted := false
	for path, file := range ctx.dirfilesout.Directories {
		highlight, trend := getTrend(ctx, file.Current.Count, file.Histories)
		highlight = highlight || (*ctx.replay && file.Current.Count > 0)
		ctx.fileprocessed = ctx.fileprocessed + uint64(file.Current.Count)
		paths := strings.Split(strings.ToLower(path), "\\")
		class := "common"
		delta := 0
		if len(file.Histories) > 0 {
			delta = file.Current.Count - file.Histories[len(file.Histories)-1].Count
		}
		if highlight {
			highlighted = true
			if file.Current.Count == 0 {
				color.Set(color.FgHiGreen)
				class = "empty"
			} else if len(file.Histories) > 0 {
				if file.Histories[len(file.Histories)-1].Count == 0 {
					color.Set(color.FgHiYellow)
					class = "recent"
				} else if file.Histories[len(file.Histories)-1].Count < file.Current.Count {
					color.Set(color.FgHiMagenta)
					class = "increase"
				} else {
					color.Set(color.FgHiWhite)
					class = "flat"
				}
			} else {
				color.Set(color.FgHiWhite)
				class = "flat"
			}
		}
		// ctx.filecount = ctx.filecount + uint64(file.Current.Count)
		if !*ctx.filter0 || highlight {
			if *ctx.selectfile == "" || strings.Contains(strings.ToLower(file.Path), strings.ToLower(*ctx.selectfile)) {
				if *ctx.influxdb != "" {
					if _, err := io.WriteString(os.Stdout, fmt.Sprintf("%s,path=%s,set=%s,class=%s value=%di,delta=%di,bigger=%di,smaller=%di,older=%di,younger=%di\n",
						*ctx.influxdb,
						strings.Replace(file.Path[len(file.Base):], " ", "_", -1),
						paths[len(paths)-1],
						class,
						file.Current.Count,
						delta,
						file.Current.MoreBytes,
						file.Current.LessBytes,
						int(file.Current.MoreSecs.Seconds()),
						int(file.Current.LessSecs.Seconds()),
					)); err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
				} else {
					fmt.Printf("Directory processed : %s - %d files%s\n", file.Path, file.Current.Count, trend)
				}

				if *ctx.details != "" && *ctx.replay {
					if _, err := io.WriteString(ctx.detailsout, fmt.Sprintf("%s\t%d\t%s\n", file.Path, file.Current.Count, trend)); err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
				}
			}
		}
		if highlight {
			color.Unset()
		}
		if *ctx.verbose {
			if !*ctx.filter0 || highlight {
				if *ctx.selectfile == "" || strings.Contains(strings.ToLower(file.Path), strings.ToLower(*ctx.selectfile)) {
					file.Current.dumpDetails()
				}
			}
		}
		// ctx.fileprocessed++
	}
	ctx.endtime = time.Now()
	if *ctx.verbose {
		if highlighted {
			fmt.Println("Legend:")
			color.Set(color.FgHiGreen)
			fmt.Println("Got 0 file")
			color.Set(color.FgHiYellow)
			fmt.Println("Got new file(s) but was empty")
			color.Set(color.FgHiMagenta)
			fmt.Println("Increase pending file(s)")
			color.Set(color.FgHiWhite)
			fmt.Println("No new file but pending exist")
			color.Unset()
		}
		elapsedtime := ctx.endtime.Sub(ctx.starttime)
		seconds := int64(elapsedtime.Seconds())
		if seconds == 0 {
			seconds = 1
		}
		fmt.Printf("**END** (%v)\n  REPORT:\n  - Elapsed time: %v\n  - Files/Dirs: %d processed on f/d(%d/%d)\n",
			ctx.endtime,
			elapsedtime,
			ctx.fileprocessed,
			ctx.filecount,
			ctx.dircount,
		)
	}
	return
}

// if we had a Json file, and in a quickrefresh, we'll use the file entries
//
func listCount(ctx *context) bool {
	var haserror bool
	if *ctx.verbose {
		fmt.Printf("Quick Process - %d Directories\n", len(ctx.dirfilesout.Directories))
	}
	if *ctx.replay {
		if *ctx.verbose {
			ctx.dircount = uint64(len(ctx.dirfilesout.Directories))
			for _, item := range ctx.dirfilesout.Directories {
				ctx.filecount = ctx.filecount + uint64(item.Current.Count)
			}
			fmt.Println("Read Quick list")
		}
	} else {
		for i, dir := range ctx.dirfilesout.Directories {
			if p_debug {
				fmt.Printf("Refresh Quick list %s %d\n", dir.Path, dir.Current.Count)
			}
			ctx.dircount++
			files, err := ioutil.ReadDir(dir.Path)
			haserror = err != nil
			if len(ctx.dirfilesout.Directories[i].Histories) >= *ctx.history {
				neededHistories := dir.Histories[1:]
				copiedHistories := make([]Stat, *ctx.history-1)
				copy(copiedHistories, neededHistories)
				dir.Histories = copiedHistories
			}
			dir.Histories = append(dir.Histories, dir.Current)
			curr := Stat{Count: 0, MoreSecs: math.MinInt64, LessSecs: math.MaxInt64, MoreBytes: math.MinInt64, LessBytes: math.MaxInt64}
			for _, file := range files {
				if !file.IsDir() {
					if *ctx.details != "" {
						if _, err := io.WriteString(ctx.detailsout, fmt.Sprintf("%s\t%s\t%v\t%d\n", dir.Path, file.Name(), file.ModTime(), file.Size())); err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
					}
					curr = curr.registerFile(file)
					ctx.filecount++
					if uint64(*ctx.feedback) > 0 && ctx.filecount%uint64(*ctx.feedback) == 0 {
						fmt.Printf("f/d(%d/%d)\r", ctx.filecount, ctx.dircount)
					}
				}
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
	dir := map[string]string{}
	specs := strings.Split(*ctx.src, ";")
	for i := 0; i < len(specs); i++ {
		if isWildcard(specs[i]) {
			if err := getFiles(ctx, specs[i]); err != nil {
				haserror = true
				fmt.Errorf("Process error:", err)
			}
		} else if strings.HasSuffix(specs[i], "\\") {
			paths := strings.Split(specs[i], "\\")
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
				if dir[base] != "" {
					dir[base] = dir[base] + ";" + lookfor
				} else {
					dir[base] = lookfor
				}
			} else {
				haserror = true
				fmt.Errorf("Process error")
			}

		} else {
			if err := getFiles(ctx, specs[i]); err != nil {
				haserror = true
				fmt.Errorf("Process error:", err)
			}
		}
	}
	for p, look := range dir {
		if *ctx.verbose {
			fmt.Printf("processing path %s looking for %s\n", p, look)
		}
		if err := getFilesInPath(ctx, p, look); err != nil {
			haserror = true
			fmt.Errorf("Process error for path [%s] looking for %s", p, look)
		}
	}

	fixedCount(ctx)
	return haserror
}

func initDataArea(ctx *context) {
	ctx.allfilesout = make([]os.FileInfo, 0, 300)
	ctx.dirfilesout = Directories{Src: *ctx.src, Directories: map[string]Directory{}}
}

func getConfig(ctx *context) bool {
	file, err := os.Open(*ctx.quick)
	if err != nil {
		return false
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	Dir := Directories{}
	err = decoder.Decode(&Dir)
	if err != nil {
		fmt.Println("error:", err)
		return false
	}
	if *ctx.replay {
		ctx.src = &Dir.Src
	}

	if strings.ToLower(Dir.Src) != strings.ToLower(*ctx.src) {
		fmt.Println("***Start from empty file. Different Src args***")
		return false
	}
	for _, onedir := range Dir.Directories {
		ctx.dirfilesout.Directories[onedir.Path] = onedir
	}
	return true
}

// VersionNum : Litteral version
// 1.0 : Original
// 1.1 : Highlight important data
// 1.2 : Optimization on first discovery. Walk already work on files. So use Walk file entry
// 1.3 : Feedback on directory count
// 1.4 : Dump fichiers dans CSV (Tab)
// 1.5 : Dump fichiers dans CSV (Tab) pour le mode replay & Couleur+Legende
// 1.6 : Ajout des erreurs dans un fichier dump. Erreur non fatal dans Walk
// 1.7 : Option influxdb pour sortir sur le Standard Output les données InfluxDB
// 1.8 : Ajout de Treesize
const VersionNum = "1.8"

func main() {
	if err := processArgs(&contexte); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	var err error
	if *contexte.details != "" {
		contexte.detailsout, err = os.Create(*contexte.details)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		defer contexte.detailsout.Close()

		if *contexte.replay {
			if _, err := io.WriteString(contexte.detailsout, fmt.Sprintf("%s\t%s\t%s\n", "path", "filecount", "trend")); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else if *contexte.flagtree {
			if _, err := io.WriteString(contexte.detailsout, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", "base", "path", "filecount", "totalsize", "size", "youngest_min", "youngest", "oldest_min", "oldest")); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		} else {
			if _, err := io.WriteString(contexte.detailsout, fmt.Sprintf("%s\t%s\t%s\t%s\n", "path", "name", "modified", "size")); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
	}

	if *contexte.errors != "" {
		contexte.errorsout, err = os.Create(*contexte.errors)
		if err != nil {
			fmt.Println(err)
			os.Exit(3)
		}
		defer contexte.errorsout.Close()
	}

	contexte.starttime = time.Now()

	if *contexte.quick != "" {
		contexte.processlist = getConfig(&contexte)
	}
	initDataArea(&contexte)

	if contexte.processlist {
		if !listCount(&contexte) {
			if *contexte.verbose {
				fmt.Println("\nWITH PROCESS ERROR\n") // handle error
			}
		}

	} else {
		if !genericCount(&contexte) {
			if *contexte.verbose {
				fmt.Println("\nWITH PROCESS ERROR\n") // handle error
			}
		}
	}

	if *contexte.quick != "" && !*contexte.replay {
		dirs := Directories(contexte.dirfilesout)
		dirsJson, _ := json.Marshal(dirs)
		ioutil.WriteFile(*contexte.quick, dirsJson, 0644)
	}

	os.Exit(0)
}
