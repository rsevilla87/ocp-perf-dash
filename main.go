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

	"github.com/kube-burner/kube-burner/v2/pkg/burner"
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
	Name      string
	Runs      []Run
	Path      string
	Workloads []Workload
}

type Workload struct {
	Name     string
	Path     string
	Job      string
	RunCount int
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

type MetricGroup struct {
	MetricName string
	Charts     []ChartData
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
	path := strings.TrimPrefix(r.URL.Path, "/job/")
	pathParts := strings.Split(path, "/")

	var jobName, workloadName string
	if len(pathParts) >= 1 {
		jobName = pathParts[0]
	}
	if len(pathParts) >= 2 {
		workloadName = pathParts[1]
	}

	job := Job{
		Name: jobName,
	}
	job.Path = filepath.Join(c.resultsDir, jobName)

	// Load workloads for this job
	job.Workloads, err = loadWorkloads(job.Path, jobName)
	if err != nil {
		fmt.Printf("Error loading workloads for job %s: %v\n", jobName, err)
	}

	// Determine the path to load runs from
	var runsPath string
	var displayName string
	if workloadName != "" {
		runsPath = filepath.Join(job.Path, workloadName)
		displayName = fmt.Sprintf("%s / %s", jobName, workloadName)
	} else {
		// If no workload specified, check if there are workloads
		// If there's only one workload, redirect to it
		if len(job.Workloads) == 1 {
			http.Redirect(w, r, fmt.Sprintf("/job/%s/%s", jobName, job.Workloads[0].Name), http.StatusFound)
			return
		}
		// Otherwise, show workload selection (we'll handle this in the template)
		runsPath = job.Path
		displayName = jobName
	}
	if workloadName != "" {
		job.Runs, err = loadRuns(runsPath)
	}

	metricGroups := prepareChartData(&job)
	type TemplateData struct {
		Job              Job
		WorkloadName     string
		DisplayName      string
		MetricGroups     []MetricGroup
		MetricGroupsJSON template.JS
	}

	metricGroupsJSON, _ := json.Marshal(metricGroups)

	data := TemplateData{
		Job:              job,
		WorkloadName:     workloadName,
		DisplayName:      displayName,
		MetricGroups:     metricGroups,
		MetricGroupsJSON: template.JS(metricGroupsJSON),
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
			// Load workloads for each job
			job.Workloads, _ = loadWorkloads(job.Path, job.Name)
			jobs = append(jobs, job)
		}
	}

	return jobs, nil
}

func loadWorkloads(jobPath string, jobName string) ([]Workload, error) {
	entries, err := os.ReadDir(jobPath)
	if err != nil {
		return nil, err
	}

	var workloads []Workload
	for _, entry := range entries {
		if entry.IsDir() {
			workloadPath := filepath.Join(jobPath, entry.Name())
			// Count runs without loading all the data
			runCount := countRuns(workloadPath)
			workloads = append(workloads, Workload{
				Name:     entry.Name(),
				Path:     workloadPath,
				Job:      jobName,
				RunCount: runCount,
			})
		}
	}

	return workloads, nil
}

func countRuns(workloadPath string) int {
	entries, err := os.ReadDir(workloadPath)
	if err != nil {
		return 0
	}
	return len(entries)
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
		return burner.JobSummary{}, err
	}

	err = json.Unmarshal(data, &summaries)
	if err != nil {
		return burner.JobSummary{}, err
	}

	if len(summaries) == 0 {
		return burner.JobSummary{}, fmt.Errorf("no job summary found")
	}
	return burner.JobSummary{}, nil
}

func prepareChartData(job *Job) []MetricGroup {
	// First, group by metricName, then by quantileName
	// Map structure: metricName -> quantileName -> []DataPoint
	metricMap := make(map[string]map[string][]DataPoint)

	for _, run := range job.Runs {
		for _, measurement := range run.Measurements {
			metricName := measurement.MetricName
			quantileName := measurement.QuantileName

			// Initialize metric map if needed
			if metricMap[metricName] == nil {
				metricMap[metricName] = make(map[string][]DataPoint)
			}

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
			metricMap[metricName][quantileName] = append(metricMap[metricName][quantileName], dataPoint)
		}
	}

	// Create MetricGroup for each metricName
	var metricGroups []MetricGroup
	for metricName, quantileMap := range metricMap {
		var charts []ChartData
		for quantileName, datapoints := range quantileMap {
			sort.Slice(datapoints, func(i, j int) bool {
				return datapoints[i].Timestamp.Before(datapoints[j].Timestamp)
			})

			charts = append(charts, ChartData{
				MetricName:   metricName,
				QuantileName: quantileName,
				Datapoints:   datapoints,
			})
		}

		// Sort charts by quantileName
		sort.Slice(charts, func(i, j int) bool {
			return charts[i].QuantileName < charts[j].QuantileName
		})

		metricGroups = append(metricGroups, MetricGroup{
			MetricName: metricName,
			Charts:     charts,
		})
	}

	// Sort metric groups by metricName
	sort.Slice(metricGroups, func(i, j int) bool {
		return metricGroups[i].MetricName < metricGroups[j].MetricName
	})

	return metricGroups
}
