package commands

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"coderaft/internal/ui"
)

type portMapping struct {
	ContainerPort string
	HostPort      string
	Protocol      string
	URL           string
	Service       string
}

var commonPorts = map[int]string{
	22:    "SSH",
	80:    "HTTP",
	443:   "HTTPS",
	3000:  "Dev Server",
	3001:  "Alt Dev",
	4000:  "Phoenix",
	5000:  "Flask/Dev",
	5173:  "Vite",
	5432:  "PostgreSQL",
	5500:  "Live Server",
	5555:  "Flower",
	6379:  "Redis",
	8000:  "Django/Uvicorn",
	8080:  "HTTP Alt",
	8081:  "HTTP Alt",
	8443:  "HTTPS Alt",
	8888:  "Jupyter",
	9000:  "PHP-FPM",
	9229:  "Node Debug",
	9292:  "Rack",
	27017: "MongoDB",
}

var portsCmd = &cobra.Command{
	Use:   "ports [project]",
	Short: "Show port mappings for running islands",
	Long: `Display all forwarded ports for coderaft islands with clickable URLs.

Examples:
  coderaft ports              # Show ports for all running islands
  coderaft ports myproject    # Show ports for a specific project`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return showProjectPorts(args[0])
		}
		return showAllPorts()
	},
}

var portsWatchCmd = &cobra.Command{
	Use:   "watch [project]",
	Short: "Watch for port changes in real-time",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := ""
		if len(args) == 1 {
			project = args[0]
		}
		return watchPorts(project)
	},
}

func init() {
	portsCmd.AddCommand(portsWatchCmd)
}

func showAllPorts() error {
	islands, err := dockerClient.ListIslands()
	if err != nil {
		return fmt.Errorf("failed to list islands: %w", err)
	}

	runningIslands := []string{}
	for _, b := range islands {
		if strings.Contains(strings.ToLower(b.Status), "up") {
			name := ""
			if len(b.Names) > 0 {
				name = strings.TrimPrefix(b.Names[0], "/")
			}
			if name != "" {
				runningIslands = append(runningIslands, name)
			}
		}
	}

	if len(runningIslands) == 0 {
		ui.Info("no running islands found.")
		ui.Info("start an island with 'coderaft up' or 'coderaft shell <project>'")
		return nil
	}

	hasAnyPorts := false
	for _, island := range runningIslands {
		ports, err := dockerClient.GetPortMappings(island)
		if err != nil {
			continue
		}
		if len(ports) > 0 {
			hasAnyPorts = true
			break
		}
	}

	if !hasAnyPorts {
		ui.Info("no ports exposed on running islands.")
		ui.Info("configure ports in coderaft.json: \"ports\": [\"3000:3000\", \"8080:8080\"]")
		return nil
	}

	ui.Header("exposed ports")
	ui.Blank()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tPORT\tURL\tSERVICE")
	fmt.Fprintln(w, "-------\t----\t---\t-------")

	for _, island := range runningIslands {
		projectName := strings.TrimPrefix(island, "coderaft_")
		ports, err := dockerClient.GetPortMappings(island)
		if err != nil || len(ports) == 0 {
			continue
		}

		mappings := parsePorts(ports)
		for _, m := range mappings {
			fmt.Fprintf(w, "%s\t%s→%s\t%s\t%s\n",
				projectName,
				m.HostPort,
				m.ContainerPort,
				m.URL,
				m.Service,
			)
		}
	}
	w.Flush()

	ui.Blank()
	ui.Info("tip: click URLs to open in browser (terminal-dependent)")

	return nil
}

func showProjectPorts(projectName string) error {
	if err := validateProjectName(projectName); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	islandName := fmt.Sprintf("coderaft_%s", projectName)

	exists, err := dockerClient.IslandExists(islandName)
	if err != nil {
		return fmt.Errorf("failed to check island: %w", err)
	}
	if !exists {
		return fmt.Errorf("island '%s' not found. Run 'coderaft up' first", projectName)
	}

	status, err := dockerClient.GetIslandStatus(islandName)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}
	if status != "running" {
		return fmt.Errorf("island '%s' is not running (status: %s)", projectName, status)
	}

	ports, err := dockerClient.GetPortMappings(islandName)
	if err != nil {
		return fmt.Errorf("failed to get ports: %w", err)
	}

	if len(ports) == 0 {
		ui.Info("no ports exposed for '%s'.", projectName)
		ui.Blank()
		ui.Info("to expose ports, add to coderaft.json:")
		ui.Info("  \"ports\": [\"3000:3000\", \"8080:8080\"]")
		return nil
	}

	mappings := parsePorts(ports)

	ui.Header("ports for %s", projectName)
	ui.Blank()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HOST\tCONTAINER\tURL\tSERVICE")
	fmt.Fprintln(w, "----\t---------\t---\t-------")

	for _, m := range mappings {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			m.HostPort,
			m.ContainerPort,
			m.URL,
			m.Service,
		)
	}
	w.Flush()

	ui.Blank()

	// Show quick links
	if len(mappings) > 0 {
		ui.Info("quick access:")
		for _, m := range mappings {
			if m.URL != "" {
				ui.Item("%s  %s", m.URL, m.Service)
			}
		}
	}

	return nil
}

func parsePorts(rawPorts []string) []portMapping {
	mappings := []portMapping{}

	// Pattern: "0.0.0.0:3000->3000/tcp" or "3000/tcp" or "8080:80"
	re := regexp.MustCompile(`(?:(\d+\.\d+\.\d+\.\d+):)?(\d+)(?:->(\d+))?(?:/(\w+))?`)

	for _, raw := range rawPorts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		matches := re.FindStringSubmatch(raw)
		if len(matches) == 0 {
			// Fallback: try simple "host:container" format
			parts := strings.Split(raw, ":")
			if len(parts) == 2 {
				hostPort := strings.TrimSpace(parts[0])
				containerPort := strings.TrimSpace(parts[1])
				mappings = append(mappings, portMapping{
					HostPort:      hostPort,
					ContainerPort: containerPort,
					Protocol:      "tcp",
					URL:           fmt.Sprintf("http://localhost:%s", hostPort),
					Service:       guessService(hostPort),
				})
			}
			continue
		}

		hostPort := matches[2]
		containerPort := matches[3]
		protocol := matches[4]

		if containerPort == "" {
			containerPort = hostPort
		}
		if protocol == "" {
			protocol = "tcp"
		}

		m := portMapping{
			HostPort:      hostPort,
			ContainerPort: containerPort,
			Protocol:      protocol,
			Service:       guessService(containerPort),
		}

		// Generate URL for HTTP-friendly ports
		if protocol == "tcp" {
			port, _ := strconv.Atoi(hostPort)
			if isHTTPPort(port) {
				m.URL = fmt.Sprintf("http://localhost:%s", hostPort)
			} else if port == 443 || port == 8443 {
				m.URL = fmt.Sprintf("https://localhost:%s", hostPort)
			}
		}

		mappings = append(mappings, m)
	}

	// Sort by host port
	sort.Slice(mappings, func(i, j int) bool {
		pi, _ := strconv.Atoi(mappings[i].HostPort)
		pj, _ := strconv.Atoi(mappings[j].HostPort)
		return pi < pj
	})

	return mappings
}

func guessService(port string) string {
	p, err := strconv.Atoi(port)
	if err != nil {
		return ""
	}

	if name, ok := commonPorts[p]; ok {
		return name
	}

	return ""
}

func isHTTPPort(port int) bool {
	httpPorts := []int{80, 3000, 3001, 4000, 5000, 5173, 5500, 8000, 8080, 8081, 8888, 9000, 9292}
	for _, p := range httpPorts {
		if port == p {
			return true
		}
	}
	// Also consider any port in the common web range
	if port >= 3000 && port <= 9999 {
		return true
	}
	return false
}

func watchPorts(project string) error {
	ui.Info("watching for port changes... (press Ctrl+C to stop)")
	ui.Blank()

	// Initial display
	if project != "" {
		if err := showProjectPorts(project); err != nil {
			ui.Warning("%v", err)
		}
	} else {
		if err := showAllPorts(); err != nil {
			ui.Warning("%v", err)
		}
	}

	ui.Blank()
	ui.Info("port watching requires manual refresh in this version.")
	ui.Info("run 'coderaft ports' again to see updated mappings.")

	return nil
}
