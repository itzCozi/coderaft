#!/bin/bash




BOX_NAME="${DEVBOX_BOX_NAME:-unknown}"
PROJECT_NAME="${DEVBOX_PROJECT_NAME:-unknown}"

case "$1" in
    "exit"|"quit")
        echo "👋 Exiting devbox shell for project '$PROJECT_NAME'"
        exit 0
        ;;
    "status"|"info")
        echo "📊 Devbox Box Status"
        echo "Project: $PROJECT_NAME"
        echo "Box: $BOX_NAME"
        echo "Workspace: /workspace"
        echo "Host: $(cat /etc/hostname)"
        echo "User: $(whoami)"
        echo "Working Directory: $(pwd)"
        echo ""
        echo "💡 Available devbox commands inside box:"
        echo "  devbox exit     - Exit the shell"
        echo "  devbox status   - Show box information"
        echo "  devbox help     - Show this help"
        ;;
    "help"|"--help"|"-h")
        echo "🚀 Devbox box Commands"
        echo ""
        echo "Available commands inside the box:"
        echo "  devbox exit         - Exit the devbox shell"
        echo "  devbox status       - Show box and project information"
        echo "  devbox help         - Show this help message"
        echo ""
        echo "📁 Your project files are in: /workspace"
        echo "🐧 You're in an Ubuntu box with full package management"
        echo ""
        echo "Examples:"
        echo "  devbox exit                    # Exit to host"
        echo "  devbox status                  # Check box info"
        echo ""
        echo "💡 Tip: Files in /workspace are shared with your host system"
        ;;
    "host")
        echo "⚠️  The 'devbox host' command is not yet available."
        echo "To run commands on the host, exit the box first with 'devbox exit'."
        exit 1
        ;;
    "version")
        echo "devbox box wrapper v1.0"
        echo "Box: $BOX_NAME"
        echo "Project: $PROJECT_NAME"
        ;;
    "")
        echo "❌ Missing command. Use 'devbox help' for available commands."
        exit 1
        ;;
    *)
        echo "❌ Unknown devbox command: $1"
        echo "💡 Use 'devbox help' to see available commands inside the box"
        echo ""
        echo "Available commands:"
        echo "  exit, status, help, version"
        exit 1
        ;;
esac
