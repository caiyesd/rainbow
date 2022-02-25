package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/tsingson/cpuaffinity"
)

type Quality interface{}
type Pressure interface{}

type Monitor interface {
	Start() error
	Stop() error
	Quality() Quality
}

type Presser interface {
	Start() error
	Stop() error
	SetPressure(pressure Pressure) error
	Pressure() Pressure
}

type Scheduler interface {
	SetPresser(p Presser)
	SetMonitor(m Monitor)
	SetTargetQuality(q Quality)
	SetInitialPressure(p Pressure)
	Run() error
}

// ----------------------

const MAX_CPU_CONSUMPTION = 10000

type CpuConsumer struct {
	period      time.Duration
	consumption int
	cpuId       int
}

func NewCpuConsumer(cpuId int, consumption int) *CpuConsumer {
	if consumption < 0 {
		consumption = 0
	} else if consumption > MAX_CPU_CONSUMPTION {
		consumption = MAX_CPU_CONSUMPTION
	}
	return &CpuConsumer{
		cpuId:       cpuId,
		consumption: consumption,
		period:      time.Millisecond * 10,
	}
}
func (p *CpuConsumer) Run(ctx context.Context) error {
	// if p.cpuId < 0 {
	// 	for i := 0; i < runtime.NumCPU(); i++ {
	// 		cpuId := i
	// 		go func() {
	// 			cpuaffinity.SetAffinity(cpuId)
	// 			for {
	// 				run := p.period * time.Duration(p.consumption) / MAX_CPU_CONSUMPTION
	// 				sleep := p.period * (MAX_CPU_CONSUMPTION - time.Duration(p.consumption)) / MAX_CPU_CONSUMPTION
	// 				select {
	// 				case <-ctx.Done():
	// 					return
	// 				default:
	// 				}
	// 				now := time.Now()
	// 				for time.Now().Before(now.Add(run)) {
	// 				}
	// 				time.Sleep(sleep)
	// 			}
	// 		}()
	// 	}
	// 	<-ctx.Done()
	// 	return nil
	// } else {
	cpuaffinity.SetAffinity(p.cpuId)
	for {
		run := p.period * time.Duration(p.consumption) / MAX_CPU_CONSUMPTION
		sleep := p.period * (MAX_CPU_CONSUMPTION - time.Duration(p.consumption)) / MAX_CPU_CONSUMPTION
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		now := time.Now()
		for time.Now().Before(now.Add(run)) {
		}
		time.Sleep(sleep)
	}
	// }

}
func (p *CpuConsumer) SetConsumption(consumption int) {
	p.consumption = consumption
}

func (p *CpuConsumer) Consumption() int {
	return p.consumption
}

// ----------------------

type CpuStat struct {
	id         int
	user       uint64
	nice       uint64
	system     uint64
	idle       uint64
	iowait     uint64
	irq        uint64
	softirq    uint64
	steal      uint64
	guest      uint64
	guest_nice uint64
}

func (s *CpuStat) Parse(line string) error {
	words := strings.Split(strings.ReplaceAll(line, "  ", " "), " ")

	if len(words) < 11 {
		return fmt.Errorf("invalid cpu stat: %s", line)
	}
	if !strings.HasPrefix(words[0], "cpu") {
	}
	id := -1
	if words[0] != "cpu" {
		_id, err := strconv.ParseInt(words[0][3:], 10, 32)
		if err != nil {
			return err
		}
		id = int(_id)
	}

	user, err := strconv.ParseUint(words[1], 10, 64)
	if err != nil {
		return err
	}

	nice, err := strconv.ParseUint(words[2], 10, 64)
	if err != nil {
		return err
	}

	system, err := strconv.ParseUint(words[3], 10, 64)
	if err != nil {
		return err
	}

	idle, err := strconv.ParseUint(words[4], 10, 64)
	if err != nil {
		return err
	}

	iowait, err := strconv.ParseUint(words[5], 10, 64)
	if err != nil {
		return err
	}

	irq, err := strconv.ParseUint(words[6], 10, 64)
	if err != nil {
		return err
	}

	softirq, err := strconv.ParseUint(words[7], 10, 64)
	if err != nil {
		return err
	}

	steal, err := strconv.ParseUint(words[8], 10, 64)
	if err != nil {
		return err
	}

	guest, err := strconv.ParseUint(words[9], 10, 64)
	if err != nil {
		return err
	}

	guest_nice, err := strconv.ParseUint(words[10], 10, 64)
	if err != nil {
		return err
	}

	s.id = id
	s.user = user
	s.nice = nice
	s.system = system
	s.idle = idle
	s.iowait = iowait
	s.irq = irq
	s.softirq = softirq
	s.steal = steal
	s.guest = guest
	s.guest_nice = guest_nice
	return nil
}

func (s *CpuStat) Total() uint64 {
	return s.user + s.nice + s.system + s.idle + s.iowait + s.irq + s.softirq + s.steal + s.guest + s.guest_nice
}

const MAX_CPU_USAGE int = 10000

type CpuMonitor struct {
	next     int
	cpuStats [2][]*CpuStat
}

func NewCpuMonitor() *CpuMonitor {
	return &CpuMonitor{}
}

func (m *CpuMonitor) Sample() error {
	data, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	cpustats := make([]*CpuStat, 0)
	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		stat := new(CpuStat)
		err = stat.Parse(line)
		if err != nil {
			return err
		}
		cpustats = append(cpustats, stat)
	}
	if len(cpustats) == 0 {
		return fmt.Errorf("no cpu stat info")
	}
	m.cpuStats[m.next] = cpustats
	m.next = (m.next + 1) % 2
	return nil
}

func (m *CpuMonitor) Usage(cpuId int) int {
	if len(m.cpuStats[m.next]) != len(m.cpuStats[(m.next+1)%2]) {
		return -1
	}
	if len(m.cpuStats[m.next]) == 0 {
		return -2
	}
	if cpuId+1 >= len(m.cpuStats[m.next]) || cpuId+1 < 0 {
		return -3
	}
	idx := cpuId + 1
	total := m.cpuStats[(m.next+2-1)%2][idx].Total() - m.cpuStats[m.next][idx].Total()
	idle := m.cpuStats[(m.next+2-1)%2][idx].idle - m.cpuStats[m.next][idx].idle
	return int((total - idle) * uint64(MAX_CPU_USAGE) / total)
}

// ----------------------

func rainbow(cpuId int, targetUsage int) {
	// log.Println(runtime.NumCPU())
	if cpuId < 0 || cpuId >= runtime.NumCPU() {
		for i := 0; i < runtime.NumCPU(); i++ {
			id := i
			go stabilize(id, targetUsage-targetUsage*i/runtime.NumCPU()/2-targetUsage*i/runtime.NumCPU()/3)
		}
		exit := make(chan struct{})
		<-exit
	} else {
		stabilize(cpuId, targetUsage)
	}
}

func stabilize(cpuId, targetUsage int) {
	p := NewCpuConsumer(cpuId, 0)
	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	_ = cancel

	m := NewCpuMonitor()
	m.Sample()
	for {
		time.Sleep(time.Millisecond * 100)
		m.Sample()
		usage := m.Usage(cpuId)
		consumption := p.consumption

		consumption2 := consumption
		// log.Println(usage, targetUsage-(MAX_CPU_USAGE/200), targetUsage+(MAX_CPU_USAGE/200))
		if usage > targetUsage+(MAX_CPU_USAGE/200) {
			consumption2 = consumption - (MAX_CPU_CONSUMPTION*(usage-targetUsage+(MAX_CPU_USAGE/200))/MAX_CPU_USAGE)/5
		} else if usage < targetUsage-(MAX_CPU_USAGE/200) {
			consumption2 = consumption + (MAX_CPU_CONSUMPTION*(targetUsage+(MAX_CPU_USAGE/200)-usage)/MAX_CPU_USAGE)/5
		}

		if consumption2 > MAX_CPU_CONSUMPTION {
			consumption2 = MAX_CPU_CONSUMPTION
		} else if consumption < 0 {
			consumption2 = 0
		}
		// log.Printf("usage:%d consumption:%d -> %d\n", usage, consumption, consumption2)
		if consumption2 != consumption {
			// log.Printf("[cpu%d] usage:%d consumption:%d -> %d\n", cpuId, usage, consumption, consumption2)
			p.SetConsumption(consumption2)
		}
	}
}

func main() {
	rainbow(-1, 9000)
}
