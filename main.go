// ProfessionCost by Kevin Darlington (http://outroot.com)

package main

import (
  "bufio"
  "errors"
  "fmt"
  "github.com/PuerkitoBio/goquery"
  "io"
  "log"
  "math"
  "menteslibres.net/gosexy/to"
  "net/http"
  "os"
  "path/filepath"
  "regexp"
  "sort"
  "strconv"
  "strings"
  "text/tabwriter"
)

var (
  VERSION         = "1.0.0"
  itemRe          = regexp.MustCompile(`(((\d+) x )?\[(.*?)\])`)
  inscriptRe      = regexp.MustCompile(`(([0-9.]+) stacks.*following: (.*))`)
  inscriptItemRet = regexp.MustCompile(`\[(.*?)\]`)
  professions     = []Profession{
    {"Alchemy", "http://wowprofessions.com/alchemy-leveling-guide-1-600/", ParseGeneric},
    {"Blacksmithing", "http://wowprofessions.com/blacksmithing-leveling-guide-1-600/", ParseGeneric},
    {"Enchanting", "http://wowprofessions.com/enchanting-leveling-guide-1-600-2/", ParseGeneric},
    {"Engineering", "http://wowprofessions.com/engineering-leveling-guide-1-600/", ParseGeneric},
    {"Inscription", "http://wowprofessions.com/inscription-leveling-guide-1-600/", ParseInscription},
    {"Jewelcrafting", "http://wowprofessions.com/jewelcrafting-leveling-guide-1-600/", ParseGeneric},
    {"Leatherworking", "http://wowprofessions.com/leatherworking-leveling-guide-1-600/", ParseGeneric},
    {"Tailoring", "http://wowprofessions.com/tailoring-leveling-guide-1-600/", ParseGeneric},
  }
)

// Final result of a profession and its cost
type Result struct {
  Name     string
  Low      int
  High     int
  Median   int
  Average  int
  NotFound []string
}

// Final results. Used so we can sort by price.
type Results []Result

func (r *Results) Len() int {
  return len(*r)
}

func (r *Results) Less(i, j int) bool {
  return (*r)[i].Average < (*r)[j].Average
}

func (r *Results) Swap(i, j int) {
  (*r)[i], (*r)[j] = (*r)[j], (*r)[i]
}

// The profession, the reagent url, and the reagent parser
type Profession struct {
  Name   string
  Url    string
  Parser ReagentParser
}

// A reagent. It's just its name and the count.
type Reagent struct {
  Name  string
  Count int
}

// Defines a reagent parser. The reason it's a slice
// of slices, is because some reagents should be combined
// into one price. Looking at this example from wowprofessions.com:
//   60 x [Blackened Dragonscale] OR 40 x [Savage Leather] | 2 x Volatile Air | 2 x Volatile Earth | 2 x Volatile Water | 2 x Volatile Fire
// would turn into this:
//   [][]Reagent{
//     []Reagent{Reagent{"Blackend Dragonscale", 60}},
//     []Reagent{
//       Reagent{"Savage Leather", 40},
//       Reagent{"Volatile Air", 2},
//       Reagent{"Volatile Earth", 2},
//       Reagent{"Volatile Water", 2},
//       Reagent{"Volatile Fire", 2},
//     },
//   }
// which means you can either buy 60 blackend dragonscale, or you have to by 40 savage leather and
// 2 volatile air and 2 volatile earth and 2 volatile water and 2 volatile fire.
type ReagentParser func(string) [][]Reagent

// A generic parser for wowprofessions.com.
func ParseGeneric(s string) [][]Reagent {
  allmatches := itemRe.FindAllStringSubmatch(s, -1)
  var ret [][]Reagent
  //ret := make([][]Reagent, len(allmatches))
  for _, matches := range allmatches {
    // fmt.Println(matches)
    count := int(to.Int64(matches[len(matches)-2]))
    if count == 0 {
      count = 1
    }
    name := strings.Trim(matches[len(matches)-1], " ")

    ret = append(ret, []Reagent{Reagent{name, count}})
  }

  return ret
}

// An inscription parser for wowprofessions.com. They list the
// reagents a little different from the rest of the professions.
func ParseInscription(s string) [][]Reagent {
  var ret [][]Reagent

  allmatches := inscriptRe.FindAllStringSubmatch(s, -1)
  stacks := allmatches[0][2]
  stacksf, err := strconv.ParseFloat(stacks, 64)
  if err != nil {
    return ret
  }

  list := allmatches[0][3]
  for _, match := range inscriptItemRet.FindAllStringSubmatch(list, -1) {
    ret = append(ret, []Reagent{Reagent{match[1], int(math.Ceil(stacksf * 20))}})
  }

  return ret
}

// Represents an item in theunderminejournal data
type Item struct {
  *goquery.Selection
}

// Return the market price of an item
func (e *Item) Market() int {
  m := e.Find("market").Text()
  return int(to.Int64(m))
}

// Represents theunderminejournal data
type JournalData struct {
  *goquery.Document
}

// Return an Item by its name.
func (j *JournalData) ItemByName(s string) (Item, error) {
  el := j.Find(fmt.Sprintf(`item[name="%s"]`, s))
  if el.Length() == 0 {
    return Item{}, errors.New("Not found")
  }

  return Item{el}, nil
}

// Exists reports whether the named file or directory exists.
func Exists(name string) bool {
  if _, err := os.Stat(name); err != nil {
    if os.IsNotExist(err) {
      return false
    }
  }
  return true
}

// Return a goquery document based on the name and url.
// If a cached version exists, then use that. Otherwise
// download it.
func getProfessionDoc(name, url string) (*goquery.Document, error) {
  // doc, e := goquery.NewDocument(profession.Url)
  // doc.res
  // os.Open(

  filename := fmt.Sprintf("%s.html", name)
  path := filepath.Join("cache", filename)

  if !Exists(path) {
    res, err := http.Get(url)
    if err != nil {
      return nil, err
    }

    fp, err := os.Create(path)
    if err != nil {
      return nil, err
    }

    io.Copy(fp, res.Body)
    fp.Close()
    res.Body.Close()
  }

  fp, err := os.Open(path)
  if err != nil {
    return nil, err
  }
  doc, err := goquery.NewDocumentFromReader(fp)
  if err != nil {
    return nil, err
  }
  defer fp.Close()

  return doc, nil
}

func fatal(s interface{}) {
  log.Fatal(s)
  exit()
}

func exit() {
  fmt.Println("Press ENTER to end")
  bufio.NewReader(os.Stdin).ReadString('\n')
}

func main() {
  fmt.Printf("ProfessionCost Version %s\n\n", VERSION)

  files, err := filepath.Glob("data/*.xml")
  if err != nil {
    fatal(err)
  }

  // Run report on all xml data in data/
  for _, file := range files {
    var results Results

    datafp, err := os.Open(file)
    if err != nil {
      fatal(err)
    }

    doc, err := goquery.NewDocumentFromReader(datafp)
    if err != nil {
      fatal(err)
    }

    datafp.Close()
    jdata := JournalData{doc}

    val, _ := doc.Find("realm").First().Attr("name")
    fmt.Println(val)

    for pindex, profession := range professions {
      fmt.Printf("\r  Processing %-30s\t [%d%%]", profession.Name, int(float64(pindex)/float64(len(professions))*100))

      result := Result{Name: profession.Name}

      doc, e := getProfessionDoc(profession.Name, profession.Url)
      if e != nil {
        fatal(e.Error())
      }

      el := doc.Find("ul.circle.white").First()

      el.Find("li").Each(func(i int, s *goquery.Selection) {
        reagentsAll := profession.Parser(s.Text())
        subcosts := make([]int, len(reagentsAll))

        for i, reagents := range reagentsAll {
          for _, reagent := range reagents {
            entry, err := jdata.ItemByName(reagent.Name)
            if err != nil {
              result.NotFound = append(result.NotFound, reagent.Name)
            } else {
              subcosts[i] = reagent.Count * entry.Market()
            }
          }
        }

        sort.Ints(subcosts)
        result.Low += subcosts[0]                // Min cost
        result.High += subcosts[len(subcosts)-1] // Max cost
        if len(subcosts) > 2 {
          result.Median += subcosts[int(math.Ceil(float64(len(subcosts)/2.0)))] // Median cost
        } else {
          result.Median += (subcosts[0] + subcosts[len(subcosts)-1]) / 2
        }
      })

      result.Average = (result.Low + result.High) / 2 // Average cost

      results = append(results, result)
    }

    //fmt.Printf("\r  Processing %-30s\t [%d%%]\n", "complete!", 100)
    fmt.Println("\r  -----------------------------------------------------")

    w := new(tabwriter.Writer)
    w.Init(os.Stdout, 0, 8, 2, '\t', 0)
    fmt.Fprintln(w, "  Profession\tLow\tHigh\tMedian\tAverage")

    sort.Sort(&results)
    for _, result := range results {
      fmt.Fprintf(w, "  %s\t%dg\t%dg\t%dg\t%dg\n", result.Name,
        result.Low/100/100, result.High/100/100, result.Median/100/100, result.Average/100/100)
    }

    w.Flush()
    fmt.Println("")
  }

  exit()
}
