#!/bin/bash




ISLAND_NAME="${CODERAFT_ISLAND_NAME:-unknown}"
PROJECT_NAME="${CODERAFT_PROJECT_NAME:-unknown}"

case "$1" in
    "exit"|"quit")
        echo "Exiting coderaft shell for project '$PROJECT_NAME'"
        exit 0
        ;;
    "status"|"info")
        echo "Coderaft Island Status"
        echo "Project: $PROJECT_NAME"
        echo "Island: $ISLAND_NAME"
        echo "Files: /island"
        echo "Host: $(cat /etc/hostname)"
        echo "User: $(whoami)"
        echo "Working Directory: $(pwd)"
        echo ""
        echo "Available coderaft commands on island:"
        echo "  coderaft exit     - Exit the shell"
        echo "  coderaft status   - Show island information"
        echo "  coderaft history  - Show package history"
        echo "  coderaft files    - List project files"
        echo "  coderaft disk     - Show disk usage"
        echo "  coderaft env      - Show environment"
        echo "  coderaft help     - Show this help"
        ;;
    "help"|"--help"|"-h")
        echo "Coderaft Island Commands"
        echo ""
        echo "Available commands on the island:"
        echo "  coderaft exit         - Exit the coderaft shell"
        echo "  coderaft status       - Show island and project information"
        echo "  coderaft history      - Show recorded package install history"
        echo "  coderaft files        - List project files in /island"
        echo "  coderaft disk         - Show island disk usage"
        echo "  coderaft env          - Show coderaft environment variables"
        echo "  coderaft help         - Show this help message"
        echo ""
        echo "Your project files are in: /island"
        echo "You are on a Debian island with full package management"
        echo ""
        echo "Examples:"
        echo "  coderaft exit                    # Exit to host"
        echo "  coderaft status                  # Check island info"
        echo "  coderaft history                 # See tracked packages"
        echo "  coderaft files                   # List /island contents"
        echo ""
        echo "hint: Files in /island are shared with your host system"
        ;;
    "history"|"log")
        HISTORY_FILE="${CODERAFT_HISTORY:-/island/coderaft.history}"
        if [ -f "$HISTORY_FILE" ]; then
            echo "Recorded package history:"
            cat "$HISTORY_FILE"
        else
            echo "No package history recorded yet."
            echo "hint: Install packages with apt, pip, npm, etc. and they'll be tracked automatically"
        fi
        ;;
    "files"|"ls")
        echo "Project files (/island):"
        ls -la /island 2>/dev/null || echo "No files found in /island"
        ;;
    "disk"|"usage")
        echo "Island disk usage:"
        df -h / 2>/dev/null | head -2
        echo ""
        echo "/island usage:"
        du -sh /island 2>/dev/null || echo "Unable to calculate"
        ;;
    "env")
        echo "Coderaft environment:"
        env | grep -i CODERAFT | sort || echo "No CODERAFT variables set"
        ;;
    "host")
        echo "The 'coderaft host' command is not yet available."
        echo "To run commands on the host, exit the island first with 'coderaft exit'."
        exit 1
        ;;
    "version")
        echo "coderaft island wrapper v1.0"
        echo "Island: $ISLAND_NAME"
        echo "Project: $PROJECT_NAME"
        ;;
    "")
        echo "Missing command. Use 'coderaft help' for available commands."
        exit 1
        ;;
    *)
        echo "Unknown coderaft command: $1"
        echo "hint: Use 'coderaft help' to see available commands on the island"
        echo ""
        echo "Available commands:"
        echo "  exit, status, history, files, disk, env, help, version"
        exit 1
        ;;
esac
