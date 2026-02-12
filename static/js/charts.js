// Global variables
let chartData = [];
let currentChart = null;
let selectedQuantile = 0;

// Initialize the page
function initializePage() {
    if (typeof window.chartData !== 'undefined' && window.chartData && window.chartData.length > 0) {
        chartData = window.chartData;
        initializeQuantileDropdown();
        initializeChart();
        setupModal();
    } else {
        console.error('No chart data available or data is empty');
    }
}

function initializeQuantileDropdown() {
    const quantileSelect = document.getElementById('quantileSelect');

    if (!quantileSelect) {
        console.error('quantileSelect element not found');
        return;
    }

    // Clear existing options
    quantileSelect.innerHTML = '';

    // Add options for each quantile
    chartData.forEach((chart, index) => {
        const option = document.createElement('option');
        option.value = index;
        option.textContent = chart.QuantileName;
        if (index === 0) option.selected = true;
        quantileSelect.appendChild(option);
    });
}

function createChart(quantileData) {
    const canvas = document.getElementById('mainChart');
    const ctx = canvas.getContext('2d');
    const selectedMetric = document.getElementById('metricSelect').value;
    let metricKey = selectedMetric;

    // Handle different metric naming
    if (selectedMetric === 'min') metricKey = 'Min';
    else if (selectedMetric === 'max') metricKey = 'Max';
    else if (selectedMetric === 'avg') metricKey = 'Avg';

    // Limit to most recent 100 datapoints
    const limitedDatapoints = quantileData.Datapoints.slice(-100);
    // Store the starting index of the limited dataset for proper mapping
    const startIndex = Math.max(0, quantileData.Datapoints.length - 100);

    // Track mouse events to distinguish clicks from drags
    let mouseDownTime = 0;
    let mouseDownX = 0;
    let mouseDownY = 0;
    let wasDrag = false;

    canvas.addEventListener('mousedown', function(e) {
        mouseDownTime = Date.now();
        mouseDownX = e.clientX;
        mouseDownY = e.clientY;
        wasDrag = false;
    });

    canvas.addEventListener('mousemove', function(e) {
        if (mouseDownTime > 0) {
            const dx = Math.abs(e.clientX - mouseDownX);
            const dy = Math.abs(e.clientY - mouseDownY);
            if (dx > 5 || dy > 5) { // Moved more than 5 pixels
                wasDrag = true;
            }
        }
    });

    canvas.addEventListener('mouseup', function(e) {
        // Reset drag flag after a short delay to allow onClick to check it
        setTimeout(() => {
            wasDrag = false;
            mouseDownTime = 0;
        }, 50);
    });

    return new Chart(ctx, {
        type: 'line',
        data: {
            labels: limitedDatapoints.map(d => new Date(d.Timestamp).toLocaleDateString()),
            datasets: [{
                label: quantileData.QuantileName + ' (' + selectedMetric + ')',
                data: limitedDatapoints.map(d => d[metricKey] || 0),
                borderColor: '#EE0000',
                backgroundColor: 'rgba(238, 0, 0, 0.2)',
                fill: true,
                tension: 0.1,
                pointBackgroundColor: '#EE0000',
                pointBorderColor: '#CC0000',
                pointHoverBackgroundColor: '#CC0000',
                pointHoverBorderColor: '#AA0000'
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    display: false
                },
                tooltip: {
                    callbacks: {
                        title: function(context) {
                            return context[0].label;
                        },
                        label: function(context) {
                            return context.parsed.y + ' ms';
                        }
                    }
                },
                zoom: {
                    zoom: {
                        wheel: {
                            enabled: false,
                        },
                        pinch: {
                            enabled: false
                        },
                        drag: {
                            enabled: true,
                            modifierKey: null
                        },
                        mode: 'x',
                        scaleMode: 'x'
                    },
                    pan: {
                        enabled: true,
                        mode: 'x',
                        scaleMode: 'x',
                        modifierKey: 'shift'
                    }
                }
            },
            layout: {
                padding: {
                    bottom: 20
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Latency (ms)'
                    }
                },
                x: {
                    title: {
                        display: false
                    },
                    ticks: {
                        maxRotation: 45,
                        minRotation: 0
                    }
                }
            },
            onHover: (event, elements) => {
                if (elements.length > 0) {
                    // Show pointer cursor when hovering over a datapoint
                    event.native.target.style.cursor = 'pointer';
                } else {
                    // Show crosshair cursor when not over a datapoint
                    event.native.target.style.cursor = 'crosshair';
                }
            },
            onClick: (event, elements) => {
                // Only show job summary if it was a click (not a drag/zoom)
                // Check if this was a zoom operation by checking if mouse was moved significantly
                const clickDuration = mouseDownTime > 0 ? Date.now() - mouseDownTime : 0;
                const isZoomOperation = wasDrag || clickDuration > 200;
                if (!isZoomOperation && elements.length > 0) {
                    // Use Chart.js's method to get the element at the click position
                    // This works correctly even after zooming
                    const chart = event.chart;
                    const activePoints = chart.getElementsAtEventForMode(event.native, 'nearest', { intersect: true }, true);
                    if (activePoints.length > 0) {
                        const pointIndex = activePoints[0].index;
                        // The data array corresponds to limitedDatapoints
                        // After zoom, Chart.js still uses the same data array, just shows a subset
                        if (pointIndex >= 0 && pointIndex < limitedDatapoints.length) {
                            const datapoint = limitedDatapoints[pointIndex];
                            showJobSummary(datapoint.JobSummary, datapoint.Timestamp);
                        }
                    }
                }
            }
        }
    });
}

// Initialize single chart
function initializeChart() {
    if (chartData.length > 0) {
        selectedQuantile = 0;
        updateChart();
    }
}

function updateChart() {
    // Destroy existing chart if it exists
    if (currentChart) {
        currentChart.destroy();
    }

    // Create new chart with current quantile data
    if (chartData[selectedQuantile]) {
        currentChart = createChart(chartData[selectedQuantile]);
        updateChartTitle();
        setupZoomControls();
    }
}

function setupZoomControls() {
    const resetZoomBtn = document.getElementById('resetZoom');
    if (resetZoomBtn && currentChart) {
        resetZoomBtn.onclick = function() {
            currentChart.resetZoom();
        };
    }
}

function updateChartTitle() {
    const chartTitle = document.getElementById('chartTitle');
    const selectedMetric = document.getElementById('metricSelect').value;

    if (chartTitle && chartData[selectedQuantile]) {
        chartTitle.textContent = chartData[selectedQuantile].QuantileName + ' (' + selectedMetric + ')';
    }
}

function updateQuantileDisplay() {
    const quantileSelect = document.getElementById('quantileSelect');
    selectedQuantile = parseInt(quantileSelect.value);
    updateChart();
}

function updateCharts() {
    updateChart();
}

function showJobSummary(jobSummary, timestamp) {
    const modal = document.getElementById('jobSummaryModal');
    const modalContent = document.getElementById('modalContent');

    function formatValue(value) {
        if (typeof value === 'string') {
            return value;
        } else if (typeof value === 'number') {
            return value.toString();
        } else if (typeof value === 'boolean') {
            return value ? 'true' : 'false';
        } else {
            return JSON.stringify(value);
        }
    }

    let content = '<div class="summary-section">';
    content += '<div class="summary-title">Run Information</div>';
    content += '<div class="summary-item"><span class="summary-key">Timestamp</span><span class="summary-value">' + new Date(timestamp).toLocaleString() + '</span></div>';
    content += '</div>';

    let generalSectionStarted = false;

    // Display all fields from jobSummary
    for (const [key, value] of Object.entries(jobSummary)) {
        if (key === 'jobConfig' && typeof value === 'object' && value !== null) {
            content += '<div class="summary-section">';
            content += '<div class="summary-title">Job Configuration</div>';
            for (const [configKey, configValue] of Object.entries(value)) {
                content += '<div class="summary-item"><span class="summary-key">' + configKey + '</span><span class="summary-value">' + formatValue(configValue) + '</span></div>';
            }
            content += '</div>';
        } else if (key !== 'metricName' && key !== 'timestamp') {
            if (!generalSectionStarted) {
                content += '<div class="summary-section">';
                content += '<div class="summary-title">General</div>';
                generalSectionStarted = true;
            }
            content += '<div class="summary-item"><span class="summary-key">' + key + '</span><span class="summary-value">' + formatValue(value) + '</span></div>';
        }
    }

    if (generalSectionStarted) {
        content += '</div>';
    }

    modalContent.innerHTML = content;
    modal.style.display = 'block';
}

// Modal close functionality
function setupModal() {
    const modal = document.getElementById('jobSummaryModal');
    const closeBtn = document.getElementsByClassName('close')[0];

    if (closeBtn) {
        closeBtn.onclick = function() {
            modal.style.display = 'none';
        }
    }

    window.onclick = function(event) {
        if (event.target == modal) {
            modal.style.display = 'none';
        }
    }
}

// Page initialization is called directly from HTML after chart data is set