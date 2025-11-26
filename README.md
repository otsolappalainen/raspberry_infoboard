# Raspberry Pi Dashboard

A Go-based dashboard service mainly for Raspberry Pi based system that displays:
- **Electricity prices** (current price and 24h graph)
- **HSL bus schedules** for selected stop(s)
- **Weather data** from FMI (temperature, precipitation probability, 24h forecast)
- **Real-time clock**


## Setup

###Hardware###
Designed for **Raspberry Pi** with **Raspberry Pi Touch Display 2 7"**

### Prerequisites

- Go 1.20 or higher
- HSL API key from [digitransit.fi](https://digitransit.fi/en/developers/)

### Automated Setup (Recommended)
The automated script handles configuration, building, and setting up autostart.

1. SSH into your Raspberry Pi.
2. Clone the repository (if not already done).
3. Run the setup script:
   ```bash
   chmod +x setup.sh
   ./setup.sh
   ```
4. Follow the on-screen prompts.

### Manual Setup
If you prefer to set up everything manually:

1. **Configuration**:
   Create `config.json` in the project root:
   ```json
   {
     "port": ":8080",
     "hsl_api_url": "https://api.digitransit.fi/routing/v2/hsl/gtfs/v1",
     "hsl_api_key": "YOUR_API_KEY",
     "fmi_api_url": "https://opendata.fmi.fi/wfs",
     "spot_api_url": "https://api.spot-hinta.fi/TodayAndDayForward?region=FI&priceResolution=15",
     "weather_location": "Helsinki",
     "bus_stops": [
       {"id": "HSL:1234567", "name": "Nimi"}
     ]
   }
   ```

2. **Run the Backend**:
   ```bash
   go run .
   ```

3. **Start the Kiosk UI**:
   Make the script executable and run it:
   ```bash
   chmod +x start-dashboard.sh
   ./start-dashboard.sh
   ```

## Troubleshooting
- **Logs**: Check systemd logs for the backend service:
  ```bash
  journalctl -u rasp_dashboard.service -f
  ```
- **Manual Start**: You can try running `./start-dashboard.sh` manually to see if Chromium launches correctly.

## License

MIT
