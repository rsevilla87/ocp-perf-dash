package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kube-burner/kube-burner/pkg/burner"
)

//go:embed static
var staticFiles embed.FS

//go:embed templates
var templateFiles embed.FS

type Measurement struct {
	QuantileName string    `json:"quantileName"`
	UUID         string    `json:"uuid"`
	P99          float64   `json:"P99"`
	P95          float64   `json:"P95"`
	P50          float64   `json:"P50"`
	Min          float64   `json:"min"`
	Max          float64   `json:"max"`
	Avg          float64   `json:"avg"`
	Timestamp    time.Time `json:"timestamp"`
	MetricName   string    `json:"metricName"`
	JobName      string    `json:"jobName"`
	Metadata     any       `json:"metadata"`
}

type Config struct {
	resultsDir string
	port       int
}

type Job struct {
	Name string
	Runs []Run
	Path string
}

type Run struct {
	Measurements []Measurement
	Summary      burner.JobSummary
	Path         string
}

type ChartData struct {
	MetricName   string
	QuantileName string
	Datapoints   []DataPoint
}

type DataPoint struct {
	Timestamp  time.Time
	P99        float64
	P95        float64
	P50        float64
	Min        float64
	Max        float64
	Avg        float64
	JobSummary burner.JobSummary
}

func main() {
	resultsDir := flag.String("results-dir", "results", "Path to the directory holding results")
	port := flag.Int("port", 8080, "Port to listen on")
	flag.Parse()
	c := newConfig(
		withResultsDir(*resultsDir),
		WithListenPort(*port),
	)

	// Serve static files from embedded filesystem
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Route handlers
	http.HandleFunc("/", c.jobListHandler)
	http.HandleFunc("/job/", c.jobDetailHandler)

	fmt.Printf("Server starting on :%d\n", c.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", c.port), nil))
}

func newConfig(options ...func(*Config)) *Config {
	c := &Config{}
	for _, o := range options {
		o(c)
	}
	return c
}

func withResultsDir(resultsDir string) func(*Config) {
	return func(c *Config) {
		c.resultsDir = resultsDir
	}
}

func WithListenPort(port int) func(*Config) {
	return func(c *Config) {
		c.port = port
	}
}

func (c *Config) jobListHandler(w http.ResponseWriter, r *http.Request) {
	jobs, err := loadJobs(c.resultsDir)
	if err != nil {
		fmt.Println("Error loading jobs:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templateFS, err := fs.Sub(templateFiles, "templates")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templateData, err := fs.ReadFile(templateFS, "jobs.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := template.New("jobs.html").Parse(string(templateData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, jobs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (c *Config) jobDetailHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	fmt.Println("Job detail handler called for", r.URL.Path)
	jobName := strings.TrimPrefix(r.URL.Path, "/job/")

	job := Job{
		Name: jobName,
	}
	job.Path = filepath.Join(c.resultsDir, jobName)
	job.Runs, err = loadRuns(job.Path)
	if err != nil {
		fmt.Printf("Error loading runs for job %s: %v\n", jobName, err)
		http.Error(w, fmt.Sprintf("Error loading runs for job %s: %v", jobName, err), http.StatusNotFound)
		return
	}
	chartData := prepareChartData(&job)
	type TemplateData struct {
		Job           Job
		ChartData     []ChartData
		ChartDataJSON template.JS
	}

	chartDataJSON, _ := json.Marshal(chartData)

	data := TemplateData{
		Job:           job,
		ChartData:     chartData,
		ChartDataJSON: template.JS(chartDataJSON),
	}

	templateFS, err := fs.Sub(templateFiles, "templates")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templateData, err := fs.ReadFile(templateFS, "job_detail.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t, err := template.New("job_detail.html").Parse(string(templateData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = t.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func loadJobs(resultsDir string) ([]Job, error) {

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, err
	}

	var jobs []Job
	for _, entry := range entries {
		if entry.IsDir() {
			job := Job{
				Name: entry.Name(),
				Path: filepath.Join(resultsDir, entry.Name()),
			}
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

func loadRuns(jobPath string) ([]Run, error) {
	entries, err := os.ReadDir(jobPath)
	if err != nil {
		return nil, err
	}

	var runs []Run
	fmt.Printf("Loading %d runs from %s\n", len(entries), jobPath)
	for _, entry := range entries {
		if entry.IsDir() {
			runPath := filepath.Join(jobPath, entry.Name())
			measurements, err := loadMeasurements(runPath)
			if err != nil {
				fmt.Printf("Error loading job data: %s %v\n", runPath, err)
				continue
			}

			jobSummary, err := loadJobSummary(runPath)
			if err != nil {
				fmt.Printf("Error loading job summary: %s %v\n", runPath, err)
				continue
			}

			run := Run{
				Measurements: measurements,
				Summary:      jobSummary,
				Path:         runPath,
			}

			runs = append(runs, run)
		}
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Summary.Timestamp.Before(runs[j].Summary.Timestamp)
	})

	return runs, nil
}

func loadMeasurements(runPath string) ([]Measurement, error) {
	var allMeasurements []Measurement
	files, err := filepath.Glob(filepath.Join(runPath, "*QuantilesMeasurement*.json"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no *QuantilesMeasurement*.json files found")
	}

	// Load all QuantilesMeasurement files
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("Error reading file %s: %v\n", file, err)
			continue
		}

		var measurements []Measurement
		err = json.Unmarshal(data, &measurements)
		if err != nil {
			fmt.Printf("Error unmarshaling file %s: %v\n", file, err)
			continue
		}

		allMeasurements = append(allMeasurements, measurements...)
	}

	return allMeasurements, nil
}

func loadJobSummary(runPath string) (burner.JobSummary, error) {
	var summaries []burner.JobSummary
	summaryPath := filepath.Join(runPath, "jobSummary.json")

	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return summaries[0], err
	}

	err = json.Unmarshal(data, &summaries)
	if err != nil {
		return summaries[0], err
	}

	if len(summaries) == 0 {
		return summaries[0], fmt.Errorf("no job summary found")
	}
	return summaries[0], nil
}

func prepareChartData(job *Job) []ChartData {
	// Map key: "metricName:quantileName"
	chartMap := make(map[string][]DataPoint)

	for _, run := range job.Runs {
		for _, measurement := range run.Measurements {
			key := fmt.Sprintf("%s:%s", measurement.MetricName, measurement.QuantileName)
			dataPoint := DataPoint{
				Timestamp:  measurement.Timestamp,
				P99:        measurement.P99,
				P95:        measurement.P95,
				P50:        measurement.P50,
				Min:        measurement.Min,
				Max:        measurement.Max,
				Avg:        measurement.Avg,
				JobSummary: run.Summary,
			}
			chartMap[key] = append(chartMap[key], dataPoint)
		}
	}

	var chartData []ChartData
	for key, datapoints := range chartMap {
		// Parse key to get metricName and quantileName
		parts := strings.Split(key, ":")
		if len(parts) != 2 {
			continue
		}
		metricName := parts[0]
		quantileName := parts[1]

		sort.Slice(datapoints, func(i, j int) bool {
			return datapoints[i].Timestamp.Before(datapoints[j].Timestamp)
		})

		chartData = append(chartData, ChartData{
			MetricName:   metricName,
			QuantileName: quantileName,
			Datapoints:   datapoints,
		})
	}

	// Sort by metricName first, then by quantileName
	sort.Slice(chartData, func(i, j int) bool {
		if chartData[i].MetricName != chartData[j].MetricName {
			return chartData[i].MetricName < chartData[j].MetricName
		}
		return chartData[i].QuantileName < chartData[j].QuantileName
	})

	return chartData
}
