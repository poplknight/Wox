//go:build darwin || linux

package info

import (
	"context"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// readTopProcesses asks ps for an immediate CPU and memory ranking.
func readTopProcesses(ctx context.Context) (cpu []procUse, memory []procUse) {
	command := exec.CommandContext(ctx, "ps", "-axo", "pid=,pcpu=,rss=,comm=")
	output, err := command.Output()
	if err != nil {
		return nil, nil
	}
	processes := make([]procUse, 0, 64)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		cpuValue, cpuErr := strconv.ParseFloat(fields[1], 64)
		rssKB, rssErr := strconv.ParseUint(fields[2], 10, 64)
		if cpuErr != nil || rssErr != nil {
			continue
		}
		processes = append(processes, procUse{Name: strings.Join(fields[3:], " "), CPU: cpuValue, RSS: rssKB * 1024})
	}
	cpu = append([]procUse(nil), processes...)
	sort.SliceStable(cpu, func(i, j int) bool { return cpu[i].CPU > cpu[j].CPU })
	memory = processes
	sort.SliceStable(memory, func(i, j int) bool { return memory[i].RSS > memory[j].RSS })
	return cpu, memory
}
