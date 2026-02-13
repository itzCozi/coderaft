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
        echo "  coderaft help     - Show this help"
        ;;
    "help"|"--help"|"-h")
        echo "Coderaft Island Commands"
        echo ""
        echo "Available commands on the island:"
        echo "  coderaft exit         - Exit the coderaft shell"
        echo "  coderaft status       - Show island and project information"
        echo "  coderaft help         - Show this help message"
        echo ""
        echo "Your project files are in: /island"
        echo "You are on an Ubuntu island with full package management"
        echo ""
        echo "Examples:"
        echo "  coderaft exit                    # Exit to host"
        echo "  coderaft status                  # Check island info"
        echo ""
        echo "hint: Files in /island are shared with your host system"
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
        echo "  exit, status, help, version"
        exit 1
        ;;
esac
