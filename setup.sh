#!/bin/bash

# Raspberry Pi Dashboard Setup Script

CONFIG_FILE="config.json"
SERVICE_FILE="/etc/systemd/system/rasp_dashboard.service"
AUTOSTART_DIR="$HOME/.config/autostart"
AUTOSTART_FILE="$AUTOSTART_DIR/dashboard.desktop"
BINARY_PATH="$HOME/rasp_dashboard"
REPO_DIR="$HOME/raspberry_infoboard"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}Raspberry Pi Dashboard Setup${NC}"

# 1. Update Repository
echo -e "\n${YELLOW}Checking for updates...${NC}"
git pull
if [ $? -eq 0 ]; then
    echo -e "${GREEN}Repository updated.${NC}"
else
    echo -e "${RED}Failed to update repository. Continuing...${NC}"
fi

# 2. Build Binary
echo -e "\n${YELLOW}Building application...${NC}"
go build -o "$BINARY_PATH"
if [ $? -eq 0 ]; then
    echo -e "${GREEN}Build successful. Binary at $BINARY_PATH${NC}"
else
    echo -e "${RED}Build failed! Exiting.${NC}"
    exit 1
fi

# 3. Configuration Wizard
echo -e "\n${YELLOW}Configuration Setup${NC}"
echo "1. New Settings (Overwrites existing)"
echo "2. Edit Current Settings (Not implemented, treats as New)"
echo "3. Skip Configuration"
read -p "Select option [1-3]: " config_opt

if [ "$config_opt" == "1" ] || [ "$config_opt" == "2" ]; then
    # HSL API Key
    read -p "Enter HSL API Key (leave empty to skip/use existing if manual): " hsl_key

    # Weather Location
    echo -e "\nSelect Weather Location:"
    cities=("Espoo" "Helsinki" "Turku" "Tampere" "Oulu" "Rovaniemi")
    select city in "${cities[@]}"; do
        if [ -n "$city" ]; then
            weather_loc="$city"
            break
        else
            echo "Invalid selection"
        fi
    done
    if [ -z "$weather_loc" ]; then weather_loc="Espoo"; fi

    # Bus Stops
    echo -e "\nConfigure Bus Stops (up to 6 stops):"
    echo "Note: Enter stop codes like E2185, E1488, etc."
    stops_json=""
    stop_count=0
    
    while [ $stop_count -lt 6 ]; do
        read -p "Enter Bus Stop Code (e.g., E2185) or 'done' to finish: " stop_code
        if [ "$stop_code" == "done" ]; then
            break
        fi

        if [ -n "$stops_json" ]; then stops_json="$stops_json,"; fi
		stops_json="$stops_json {\"id\": \"$stop_code\"}"
        stop_count=$((stop_count + 1))
        echo -e "${GREEN}Stop added.${NC}"
    done

    # Generate final config.json
    cat > config.json <<EOF
{
  "port": ":8080",
  "hsl_api_url": "https://api.digitransit.fi/routing/v2/hsl/gtfs/v1",
  "hsl_api_key": "$hsl_key",
  "fmi_api_url": "https://opendata.fmi.fi/wfs",
  "spot_api_url": "https://api.spot-hinta.fi/TodayAndDayForward?region=FI&priceResolution=15",
  "weather_location": "$weather_loc",
  "bus_stops": [ $stops_json ]
}
EOF
    echo -e "${GREEN}Configuration saved to config.json${NC}"
fi

# 4. Systemd Service
echo -e "\n${YELLOW}Setting up Systemd Service...${NC}"
read -p "Install/Update Systemd service? (y/n): " install_service
if [ "$install_service" == "y" ]; then
    # Create service file content
    # Added StandardOutput/Error to journal for debugging
    cat > rasp_dashboard.service <<EOF
[Unit]
Description=Raspberry Pi Info Dashboard backend
After=network-online.target
Wants=network-online.target

[Service]
User=$USER
WorkingDirectory=$REPO_DIR
ExecStart=$BINARY_PATH
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

    sudo mv rasp_dashboard.service "$SERVICE_FILE"
    sudo systemctl daemon-reload
    sudo systemctl enable rasp_dashboard.service
    sudo systemctl restart rasp_dashboard.service
    echo -e "${GREEN}Service installed and started.${NC}"
fi

# 5. Autostart (Kiosk)
echo -e "\n${YELLOW}Setting up Frontend Autostart...${NC}"
read -p "Enable Chromium Kiosk Autostart? (y/n): " install_kiosk
if [ "$install_kiosk" == "y" ]; then
    mkdir -p "$AUTOSTART_DIR"
    
    # Make sure start script is executable
    chmod +x "$REPO_DIR/start-dashboard.sh"
    
    cat > "$AUTOSTART_FILE" <<EOF
[Desktop Entry]
Type=Application
Name=Dashboard Kiosk
Exec=$REPO_DIR/start-dashboard.sh
X-GNOME-Autostart-enabled=true
EOF
    echo -e "${GREEN}Autostart configured in $AUTOSTART_FILE${NC}"
fi

echo -e "\n${GREEN}Setup Complete!${NC}"
echo "You may need to reboot for autostart to take effect."

# 6. Start Now
echo -e "\n${YELLOW}Start Dashboard Now?${NC}"
read -p "Start the dashboard UI now? (y/n): " start_now
if [ "$start_now" == "y" ]; then
    # Ensure backend is running
    if systemctl is-active --quiet rasp_dashboard.service; then
        echo "Backend service is running."
    else
        echo "Starting backend service..."
        sudo systemctl start rasp_dashboard.service
    fi

    # Start Chromium detached using the script
    echo "Starting Dashboard UI..."
    chmod +x "$REPO_DIR/start-dashboard.sh"
    nohup "$REPO_DIR/start-dashboard.sh" >/dev/null 2>&1 &
    echo -e "${GREEN}Dashboard started! You can close this connection.${NC}"
fi
