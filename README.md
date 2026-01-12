# Tasks-Go

A fast, terminal-based task manager written in Go with a beautiful TUI interface.

## Features

- 📋 Multiple task lists with easy switching
- ✅ Task completion tracking with progress display
- 📅 Due dates and recurring tasks
- 🔄 Auto-sync between "today" list and source lists
- 🎨 Beautiful TUI with keyboard navigation
- ⚡ Fast startup (compiled binary)
- 🌟 Starship prompt integration

## Installation

### From Source

```bash
# Clone the repo
git clone https://github.com/krisitan/tasks-go.git
cd tasks-go

# Build
go build -o tasks-go .

# Install to PATH
sudo cp tasks-go /usr/local/bin/tasks
# Or for user-local install:
cp tasks-go ~/.local/bin/tasks
```

### Quick Install Script

```bash
./install.sh
```

## Usage

### Interactive Mode (TUI)

```bash
tasks l    # or tasks view
```

**Controls:**
- `↑/↓` or `j/k` - Navigate
- `Enter` or `Space` - Toggle task completion
- `a` - Add new task
- `d` - Delete task
- `L` - Switch list
- `q` or `Esc` - Quit

### CLI Commands

```bash
# Add tasks
tasks add "Buy groceries"
tasks add "Review PR +3"        # Due in 3 days
tasks add "Weekly standup +7r"  # Recurring every 7 days

# Toggle/delete tasks
tasks toggle 0       # Toggle task at index 0
tasks delete 1,2,3   # Delete multiple tasks
tasks delete 0-5     # Delete range

# List management
tasks list                  # Show all lists
tasks list add work         # Create new list
tasks list switch work      # Switch to list
tasks list switch 2         # Switch by index
tasks list delete old       # Delete list
tasks list rename old new   # Rename list

# Plain text output (no TUI)
tasks lp

# Starship integration
tasks starship  # Outputs: today(2/8)
```

## Starship Integration

Add to your `~/.config/starship.toml`:

```toml
[custom.tasks]
description = "Active task list"
command = "tasks starship"
when = "test -f $HOME/.current-tasks-list"
format = "[$symbol$output]($style) "
symbol = "📋 "
style = "#e5c07b"
shell = ["bash", "--noprofile", "--norc"]
```

## File Format

Tasks are stored in `~/tasks-lists/*.md` with this format:

```
[] Task content @due:2024-01-15 @recur:7 || 2024-01-10
[x] Completed task @source:work
```

- `[]` - Pending task
- `[x]` - Completed task
- `@due:YYYY-MM-DD` - Due date
- `@recur:N` - Recurring every N days
- `@source:listname` - Source list (for today's list)
- `|| YYYY-MM-DD` - Created date

## Today's List

The "today" list is special:
- Automatically resets each day
- Tasks with due dates (today or overdue) are auto-added
- Completing a task syncs back to the source list
- Recurring tasks create new occurrences in the source list

## Switching from Bash Version

This Go version is a drop-in replacement for the bash `tasks-multi.sh` script. It uses the same file format and directory structure (`~/tasks-lists/`).

To switch:
```bash
# Backup current alias
# In ~/.zshrc, change:
# alias tasks='/path/to/tasks-multi.sh'
# To:
alias tasks='/usr/local/bin/tasks'  # or wherever you installed it
```

To switch back, just change the alias back to the bash script.

## License

MIT
