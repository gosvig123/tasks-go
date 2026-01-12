#!/bin/bash

set -e

echo "Building tasks-go..."
go build -o tasks-go .

echo ""
echo "Where would you like to install?"
echo "1) /usr/local/bin/tasks (requires sudo)"
echo "2) ~/.local/bin/tasks (user-local)"
echo "3) Don't install, just build"
echo ""
read -p "Choice [1-3]: " choice

case $choice in
    1)
        sudo cp tasks-go /usr/local/bin/tasks
        echo "✅ Installed to /usr/local/bin/tasks"
        echo ""
        echo "Add this alias to your shell config:"
        echo "  alias tasks='/usr/local/bin/tasks'"
        ;;
    2)
        mkdir -p ~/.local/bin
        cp tasks-go ~/.local/bin/tasks
        echo "✅ Installed to ~/.local/bin/tasks"
        echo ""
        echo "Make sure ~/.local/bin is in your PATH, then add alias:"
        echo "  alias tasks='~/.local/bin/tasks'"
        ;;
    3)
        echo "✅ Built ./tasks-go"
        echo ""
        echo "Run with: ./tasks-go"
        ;;
    *)
        echo "Invalid choice"
        exit 1
        ;;
esac

echo ""
echo "For Starship integration, add this to ~/.config/starship.toml:"
echo ""
cat << 'EOF'
[custom.tasks]
description = "Active task list"
command = "tasks starship"
when = "test -f $HOME/.current-tasks-list"
format = "[$symbol$output]($style) "
symbol = "📋 "
style = "#e5c07b"
shell = ["bash", "--noprofile", "--norc"]
EOF
