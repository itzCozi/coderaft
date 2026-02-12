#!/bin/bash




BOX_NAME="${CODERAFT_BOX_NAME:-unknown}"
PROJECT_NAME="${CODERAFT_PROJECT_NAME:-unknown}"

case "$1" in
    "exit"|"quit")
        echo "👋 Exiting coderaft shell for project '$PROJECT_NAME'"
        exit 0
        ;;
    "status"|"info")
        echo "📊 Coderaft Box Status"
        echo "Project: $PROJECT_NAME"
        echo "Box: $BOX_NAME"
        echo "Workspace: /workspace"
        echo "Host: $(cat /etc/hostname)"
        echo "User: $(whoami)"
        echo "Working Directory: $(pwd)"
        echo ""
        echo "💡 Available coderaft commands inside box:"
        echo "  coderaft exit     - Exit the shell"
        echo "  coderaft status   - Show box information"
        echo "  coderaft help     - Show this help"
        ;;
    "help"|"--help"|"-h")
        echo "🚀 Coderaft box Commands"
        echo ""
        echo "Available commands inside the box:"
        echo "  coderaft exit         - Exit the coderaft shell"
        echo "  coderaft status       - Show box and project information"
        echo "  coderaft help         - Show this help message"
        echo ""
        echo "📁 Your project files are in: /workspace"
        echo "🐧 You're in an Ubuntu box with full package management"
        echo ""
        echo "Examples:"
        echo "  coderaft exit                    # Exit to host"
        echo "  coderaft status                  # Check box info"
        echo ""
        echo "💡 Tip: Files in /workspace are shared with your host system"
        ;;
    "host")
        echo "⚠️  The 'coderaft host' command is not yet available."
        echo "To run commands on the host, exit the box first with 'coderaft exit'."
        exit 1
        ;;
    "version")
        echo "coderaft box wrapper v1.0"
        echo "Box: $BOX_NAME"
        echo "Project: $PROJECT_NAME"
        ;;
    "")
        echo "❌ Missing command. Use 'coderaft help' for available commands."
        exit 1
        ;;
    *)
        echo "❌ Unknown coderaft command: $1"
        echo "💡 Use 'coderaft help' to see available commands inside the box"
        echo ""
        echo "Available commands:"
        echo "  exit, status, help, version"
        exit 1
        ;;
esac
