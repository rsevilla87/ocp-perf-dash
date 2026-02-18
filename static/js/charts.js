// Global variables
let metricGroups = [];
let charts = {}; // Map of metricIndex -> Chart instance
let selectedQuantiles = {}; // Map of metricIndex -> selected quantile index
let selectedMetrics = {}; // Map of metricIndex -> selected metric

// Initialize the page
function initializePage() {
    if (typeof window.metricGroups !== 'undefined' && window.metricGroups && window.metricGroups.length > 0) {
        metricGroups = window.metricGroups;
        initializeAllCharts();
        setupModal();
    } else {
        console.error('No metric groups data available or data is empty');
    }
}

function initializeAllCharts() {
    metricGroups.forEach((metricGroup, metricIndex) => {
        initializeQuantileDropdown(metricIndex, metricGroup);
        initializeChart(metricIndex, metricGroup);
        setupZoomControls(metricIndex);
    });
}

function initializeQuantileDropdown(metricIndex, metricGroup) {
    const quantileSelect = document.getElementById(`quantileSelect-${metricIndex}`);

    if (!quantileSelect) {
        console.error(`quantileSelect-${metricIndex} element not found`);
        return;
    }

    // Clear existing options
    quantileSelect.innerHTML = '';

    // Add options for each quantile
    metricGroup.Charts.forEach((chart, index) => {
        const option = document.createElement('option');
        option.value = index;
        option.textContent = chart.QuantileName;
        if (index === 0) option.selected = true;
        quantileSelect.appendChild(option);
    });

    // Initialize selected quantile
    selectedQuantiles[metricIndex] = 0;
}

function createChart(metricIndex, quantileData, metricGroup) {
    const canvas = document.getElementById(`chart-${metricIndex}`);
    if (!canvas) {
        console.error(`Chart canvas chart-${metricIndex} not found`);
        return null;
    }

    const ctx = canvas.getContext('2d');
    const selectedMetric = selectedMetrics[metricIndex] || 'P99';
    let metricKey = selectedMetric;

    // Handle different metric naming
    if (selectedMetric === 'min') metricKey = 'Min';
    else if (selectedMetric === 'max') metricKey = 'Max';
    else if (selectedMetric === 'avg') metricKey = 'Avg';

    // Limit to most recent 100 datapoints
    const limitedDatapoints = quantileData.Datapoints.slice(-100);

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
function initializeChart(metricIndex, metricGroup) {
    if (metricGroup.Charts.length > 0) {
        selectedQuantiles[metricIndex] = 0;
        selectedMetrics[metricIndex] = 'P99';
        updateChart(metricIndex);
    }
}

function updateChart(metricIndex) {
    // Destroy existing chart if it exists
    if (charts[metricIndex]) {
        charts[metricIndex].destroy();
    }

    const metricGroup = metricGroups[metricIndex];
    if (!metricGroup || !metricGroup.Charts || metricGroup.Charts.length === 0) {
        return;
    }

    const quantileIndex = selectedQuantiles[metricIndex] || 0;
    const quantileData = metricGroup.Charts[quantileIndex];

    if (quantileData) {
        charts[metricIndex] = createChart(metricIndex, quantileData, metricGroup);
        updateChartTitle(metricIndex);
    }
}

function setupZoomControls(metricIndex) {
    const resetZoomBtn = document.querySelector(`.reset-zoom[data-metric-index="${metricIndex}"]`);
    if (resetZoomBtn && charts[metricIndex]) {
        resetZoomBtn.onclick = function() {
            if (charts[metricIndex]) {
                charts[metricIndex].resetZoom();
            }
        };
    }
}

function updateChartTitle(metricIndex) {
    const chartTitle = document.getElementById(`chartTitle-${metricIndex}`);
    const metricGroup = metricGroups[metricIndex];
    const quantileIndex = selectedQuantiles[metricIndex] || 0;
    const selectedMetric = selectedMetrics[metricIndex] || 'P99';

    if (chartTitle && metricGroup && metricGroup.Charts[quantileIndex]) {
        chartTitle.textContent = metricGroup.Charts[quantileIndex].QuantileName + ' (' + selectedMetric + ')';
    }
}

function updateQuantileDisplay(metricIndex) {
    const quantileSelect = document.getElementById(`quantileSelect-${metricIndex}`);
    if (quantileSelect) {
        selectedQuantiles[metricIndex] = parseInt(quantileSelect.value);
        updateChart(metricIndex);
    }
}

// Handle metric selection change
document.addEventListener('change', function(e) {
    if (e.target.classList.contains('metric-select')) {
        const metricIndex = parseInt(e.target.getAttribute('data-metric-index'));
        selectedMetrics[metricIndex] = e.target.value;
        updateChart(metricIndex);
    }
});

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
