const ENABLE_TUNING_CONTROLS = false;

let elecChart = null;
let weatherChart = null;
let latestData = null;
let graphFontSize = 16;
let busFontScale = 1.5;

const COLOR_PROFILE = {
    priceCenter: 15,
    priceScale: 10,
    tempCenter: 0,
    tempScale: 12,
    rainCenter: 0.6,
    rainScale: 1.8,
    midBrightness: 210,
    blue: '#1467FF',
    red: '#FF7C75',
    rainLight: '#a7c9ff',
    rainMid: '#4f8bdc',
    rainDeep: '#0d2f6f',
};

const defaultLegendLabelGenerator = Chart.defaults.plugins.legend.labels.generateLabels;

applyBusFontScale();

async function update() {
    try {
        const response = await fetch('/api/status');
        const data = await response.json();
        latestData = data;
        renderDashboard(data);
    } catch (e) {
        console.error("Update failed", e);
    }
}

function renderDashboard(data) {
    if (!data) {
        return;
    }

    // Electricity
    const currentPrice = data.electricity.current_price;
    document.getElementById('elec-current').innerText = currentPrice ? currentPrice.toFixed(2) : "--";

    // Graph: 15-min resolution
    if (data.electricity.prices) {
        const now = new Date();
        const windowEnd = new Date(now.getTime() + 24 * 60 * 60 * 1000);
        const prices = data.electricity.prices
            .map(p => ({
                ...p,
                start: new Date(p.start_time),
                end: new Date(p.end_time)
            }))
            .filter(p => p.end > now && p.start < windowEnd)
            .sort((a, b) => a.start - b.start)
            .slice(0, 96);

        const labels = prices.map(p => {
            const d = p.start;
            return d.getHours().toString().padStart(2, '0') + ":" + d.getMinutes().toString().padStart(2, '0');
        });
        const values = prices.map(p => p.price);

        const ctx = document.getElementById('elec-graph').getContext('2d');
        const barColors = values.map(getPriceColor);
        if (elecChart) {
            elecChart.destroy();
            elecChart = null;
        }
        elecChart = new Chart(ctx, {
            type: 'bar',
            data: {
                labels: labels,
                datasets: [{
                    type: 'bar',
                    label: 'Price (c/kWh)',
                    data: values,
                    backgroundColor: barColors,
                    borderWidth: 0,
                    borderRadius: 2,
                    maxBarThickness: 18,
                    categoryPercentage: 1.0,
                    barPercentage: 1.0
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: {
                        display: true,
                        grid: { color: '#222' },
                        ticks: {
                            color: '#666',
                            font: { size: graphFontSize },
                            maxRotation: 0,
                            autoSkip: true,
                            maxTicksLimit: 12
                        }
                    },
                    y: {
                        beginAtZero: true,
                        display: true,
                        grid: { color: '#222' },
                        ticks: { color: '#666', font: { size: graphFontSize } }
                    }
                }
            }
        });
    }

        // Transport
        const busList = document.getElementById('bus-list');
        busList.innerHTML = '';
        if (data.transport.stops) {
            data.transport.stops.forEach(stop => {
                const group = document.createElement('div');
                group.className = 'stop-group';
                group.innerHTML = `<div class="stop-name">${stop.stop_name}</div>`;

                if (stop.departures && stop.departures.length > 0) {
                    stop.departures.forEach(dep => {
                        const div = document.createElement('div');
                        div.className = 'bus-item';

                        const depTime = new Date(dep.time);
                        const now = new Date();
                        const diffMs = depTime - now;
                        const diffMins = Math.floor(diffMs / 60000);

                        let timeDisplay = diffMins + " min";
                        if (diffMins < 0) timeDisplay = "Now";

                        const timeClass = dep.realtime ? 'realtime' : 'scheduled';

                        div.innerHTML = `
                            <span class="bus-route">${dep.route_number}</span>
                            <span class="bus-dest">${dep.destination}</span>
                            <span class="bus-time ${timeClass}">${timeDisplay}</span>
                        `;
                        group.appendChild(div);
                    });
                } else {
                    group.innerHTML += '<div class="sub-value">No departures</div>';
                }
                busList.appendChild(group);
            });
        }

        // Weather
        if (data.weather.current) {
            document.getElementById('weather-temp').innerText = data.weather.current.temperature.toFixed(1);
            document.getElementById('weather-symbol').innerText = data.weather.current.symbol;
            if (data.weather.current.pop > 0) {
                document.getElementById('weather-pop').innerText = data.weather.current.pop.toFixed(0) + "%";
            } else {
                document.getElementById('weather-pop').innerText = "";
            }
        }

        const weatherCanvas = document.getElementById('weather-graph');
        if (weatherCanvas && data.weather.forecast) {
            const now = new Date();
            const forecast = data.weather.forecast
                .filter(wp => new Date(wp.time) >= now)
                .sort((a, b) => new Date(a.time) - new Date(b.time))
                .slice(0, 24);

            if (forecast.length === 0) {
                if (weatherChart) {
                    weatherChart.destroy();
                    weatherChart = null;
                }
                return;
            }

            const labels = forecast.map(wp => {
                const t = new Date(wp.time);
                const hours = t.getHours().toString().padStart(2, '0');
                const mins = t.getMinutes().toString().padStart(2, '0');
                return `${hours}:${mins}`;
            });
            const temps = forecast.map(wp => wp.temperature);
            const rainValues = forecast.map(wp => Math.max(0, wp.precipitation || 0));
            const tempColors = temps.map(getTempColor);
            const rainColors = rainValues.map(getRainColor);

            const ctx = weatherCanvas.getContext('2d');
            if (weatherChart) {
                weatherChart.destroy();
                weatherChart = null;
            }
            weatherChart = new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            type: 'line',
                            label: 'Temperature (°C)',
                            data: temps,
                            borderColor: tempColors,
                            backgroundColor: tempColors,
                            tension: 0.3,
                            pointRadius: 3,
                            pointBackgroundColor: tempColors,
                            pointBorderColor: tempColors,
                            yAxisID: 'yTemp',
                            fill: false,
                            segment: {
                                borderColor: ctx => {
                                    const value = ctx.p1.parsed.y;
                                    return getTempColor(value);
                                }
                            }
                        },
                        {
                            type: 'bar',
                            label: 'Rain (mm)',
                            data: rainValues,
                            backgroundColor: rainColors,
                            borderWidth: 0,
                            borderRadius: 4,
                            yAxisID: 'yRain'
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    interaction: { mode: 'index', intersect: false },
                    plugins: {
                        legend: {
                            labels: {
                                color: '#ccc',
                                usePointStyle: true,
                                boxWidth: 18,
                                boxHeight: 12,
                                generateLabels: (chart) => {
                                    const labels = defaultLegendLabelGenerator(chart);
                                    labels.forEach((label) => {
                                        if (label.text.includes('Temperature')) {
                                            label.pointStyle = 'line';
                                            label.strokeStyle = '#ccc';
                                            label.fillStyle = '#ccc';
                                        } else if (label.text.includes('Rain')) {
                                            label.pointStyle = 'rectRounded';
                                            label.fillStyle = '#ccc';
                                        }
                                    });
                                    return labels;
                                }
                            }
                        }
                    },
                    scales: {
                        x: {
                            grid: { color: '#222' },
                            ticks: { color: '#666', font: { size: graphFontSize }, maxTicksLimit: 12 }
                        },
                        yTemp: {
                            type: 'linear',
                            position: 'left',
                            grid: { color: '#222' },
                            ticks: { color: '#f3a712', font: { size: graphFontSize } }
                        },
                        yRain: {
                            type: 'linear',
                            position: 'right',
                            grid: { drawOnChartArea: false },
                            ticks: { color: '#4da6ff', font: { size: graphFontSize } },
                            beginAtZero: true,
                            suggestedMax: Math.max(...rainValues, 1)
                        }
                    }
                }
            });
        } else if (weatherChart) {
            weatherChart.destroy();
            weatherChart = null;
        }
}

function updateClock() {
    const now = new Date();
    document.getElementById('clock').innerText = now.toLocaleTimeString('fi-FI', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    document.getElementById('date').innerText = now.toLocaleDateString('fi-FI');
}

setInterval(updateClock, 1000);
updateClock();

initControls();

// Update every minute
setInterval(update, 60000);
update(); // Initial call

function getPriceColor(price) {
    if (price == null) {
        return 'rgba(255, 255, 255, 0.2)';
    }
    return sampleColorRamp(normalizeValue(price, COLOR_PROFILE.priceCenter, COLOR_PROFILE.priceScale));
}

function getTempColor(temp) {
    if (temp == null) {
        return sampleColorRamp(0.5);
    }
    return sampleColorRamp(normalizeValue(temp, COLOR_PROFILE.tempCenter, COLOR_PROFILE.tempScale));
}

function getRainColor(amount) {
    if (amount == null || amount <= 0) {
        return 'rgba(167, 201, 255, 0.2)';
    }
    return sampleRainRamp(normalizeValue(amount, COLOR_PROFILE.rainCenter, COLOR_PROFILE.rainScale));
}

function normalizeValue(value, center, scale) {
    if (!Number.isFinite(value)) {
        return 0.5;
    }
    const safeScale = Math.max(0.0001, scale);
    const relative = (value - center) / safeScale;
    return clamp01(0.5 + relative);
}

function sampleColorRamp(position) {
    const clamped = clamp01(position);
    const mid = Math.round(Math.max(0, Math.min(255, COLOR_PROFILE.midBrightness)));
    const midHex = rgbToHex(mid, mid, mid);
    if (clamped <= 0.5) {
        return blendColors(COLOR_PROFILE.blue, midHex, clamped / 0.5);
    }
    return blendColors(midHex, COLOR_PROFILE.red, (clamped - 0.5) / 0.5);
}

function sampleRainRamp(position) {
    const clamped = clamp01(position);
    if (clamped <= 0.5) {
        return blendColors(COLOR_PROFILE.rainLight, COLOR_PROFILE.rainMid, clamped / 0.5);
    }
    return blendColors(COLOR_PROFILE.rainMid, COLOR_PROFILE.rainDeep, (clamped - 0.5) / 0.5);
}

function clamp01(value) {
    return Math.max(0, Math.min(1, value));
}

function blendColors(colorA, colorB, t) {
    const start = hexToRgb(colorA);
    const end = hexToRgb(colorB);
    const ratio = clamp01(t);
    const blended = {
        r: start.r + (end.r - start.r) * ratio,
        g: start.g + (end.g - start.g) * ratio,
        b: start.b + (end.b - start.b) * ratio,
    };
    return rgbToHex(Math.round(blended.r), Math.round(blended.g), Math.round(blended.b));
}

function hexToRgb(hex) {
    let clean = hex.replace('#', '');
    if (clean.length === 3) {
        clean = clean.split('').map(ch => ch + ch).join('');
    }
    const value = parseInt(clean, 16);
    return {
        r: (value >> 16) & 255,
        g: (value >> 8) & 255,
        b: value & 255,
    };
}

function rgbToHex(r, g, b) {
    const toHex = (channel) => channelClamp(channel).toString(16).padStart(2, '0');
    return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
}

function channelClamp(value) {
    return Math.round(Math.min(Math.max(value, 0), 255));
}

function applyBusFontScale() {
    document.documentElement.style.setProperty('--bus-font-scale', busFontScale.toString());
}

function initControls() {
    const controls = document.querySelector('.controls');
    if (!ENABLE_TUNING_CONTROLS) {
        if (controls) {
            controls.style.display = 'none';
        }
        return;
    }

    const graphSlider = document.getElementById('graph-font-slider');
    const graphValue = document.getElementById('graph-font-value');
    if (graphSlider && graphValue) {
        graphSlider.value = graphFontSize;
        graphValue.textContent = `${graphFontSize}px`;
        graphSlider.addEventListener('input', (e) => {
            const sliderValue = parseInt(e.target.value, 10);
            if (Number.isNaN(sliderValue)) {
                return;
            }
            graphFontSize = sliderValue;
            graphValue.textContent = `${graphFontSize}px`;
            refreshWithCachedData();
        });
    }

    const busSlider = document.getElementById('bus-font-slider');
    const busValue = document.getElementById('bus-font-value');
    if (busSlider && busValue) {
        const initialBusValue = Math.round(busFontScale * 100);
        busSlider.value = initialBusValue;
        busValue.textContent = `${initialBusValue}%`;
        busSlider.addEventListener('input', (e) => {
            const sliderValue = parseInt(e.target.value, 10);
            if (Number.isNaN(sliderValue)) {
                return;
            }
            busFontScale = sliderValue / 100;
            busValue.textContent = `${sliderValue}%`;
            applyBusFontScale();
        });
    }

}

function refreshWithCachedData() {
    if (latestData) {
        renderDashboard(latestData);
    }
}
