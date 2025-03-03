#!/usr/bin/env bash

# Utiliy script to capture bluez traffic for debugging purposes, requires root privileges

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root"
    exit
fi
command -v dbus-monitor >/dev/null || { echo "Error: dbus-monitor is not installed."; exit 1; }
command -v btmon >/dev/null || { echo "Error: btmon is not installed."; exit 1; }
command -v mergecap >/dev/null || { echo "Error: mergecap (Wireshark) is not installed."; exit 1; }

timestamp=$(date +"%Y%m%d_%H%M%S")
dbus_log=".dbus_$timestamp.pcap"
btmon_log=".btmon_$timestamp.pcap"
merged_log="${1:-bluetooth_capture_$timestamp.pcapng}"

cleanup() {
    echo "Stopping processes..."; sudo kill $pid1 $pid2 2>/dev/null
    echo "Merging captures into $merged_log..."; mergecap -F pcapng -w "$merged_log" "$dbus_log" "$btmon_log"
    rm -f "$dbus_log" "$btmon_log"
    echo "Capture saved: $merged_log"
    exit
}

trap cleanup SIGINT SIGTERM

echo "Starting monitoring..."
dbus-monitor --pcap --system "type='signal',sender='org.bluez'" > "$dbus_log" & pid1=$!
btmon -t --write "$btmon_log" & pid2=$!

wait $pid1; wait $pid2; cleanup
