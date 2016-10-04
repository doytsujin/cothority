// The simulation cothority used for all protocols.
// This should not be used stand-alone and is only for
// the simulations. It loads the simulation-file, initialises all
// necessary hosts and starts the simulation on the root-node.
package main

import (
	"flag"
	"sync"

	"github.com/dedis/cothority/log"
	"github.com/dedis/cothority/sda"

	"github.com/dedis/cothority/monitor"
	"github.com/dedis/cothority/protocols/manage"
	// Empty imports to have the init-functions called which should
	// register the protocol
	_ "github.com/dedis/cothority/protocols"
	_ "github.com/dedis/cothority/services"
)

// The address of this host - if there is only one host in the config
// file, it will be derived from it automatically
var hostAddress string

// ip addr of the logger to connect to
var monitorAddress string

// Simul is != "" if this node needs to start a simulation of that protocol
var simul string

var debugVisible int

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.StringVar(&hostAddress, "address", "", "our address to use")
	flag.StringVar(&simul, "simul", "", "start simulating that protocol")
	flag.StringVar(&monitorAddress, "monitor", "", "remote monitor")
	flag.IntVar(&debugVisible, "debug", 1, "verbosity: 0-5")
}

// Main starts the host and will setup the protocol.
func main() {
	flag.Parse()
	log.SetDebugVisible(debugVisible)
	log.Lvl3("Flags are:", hostAddress, simul, log.DebugVisible, monitorAddress)

	scs, err := sda.LoadSimulationConfig(".", hostAddress)
	measures := make([]*monitor.CounterIOMeasure, len(scs))
	if err != nil {
		// We probably are not needed
		log.Lvl2(err, hostAddress)
		return
	}
	if monitorAddress != "" {
		if err := monitor.ConnectSink(monitorAddress); err != nil {
			log.Error("Couldn't connect monitor to sink:", err)
		}
	}
	sims := make([]sda.Simulation, len(scs))
	var rootSC *sda.SimulationConfig
	var rootSim sda.Simulation
	// having a waitgroup so the binary stops when all hosts are closed
	var wg sync.WaitGroup
	var ready = make(chan bool)
	for i, sc := range scs {
		// Starting all hosts for that server
		host := sc.Conode
		measures[i] = monitor.NewCounterIOMeasure("bandwidth", host)
		log.Lvl3(hostAddress, "Starting host", host.ServerIdentity.Address)
		// Launch a host and notifies when it's done

		wg.Add(1)
		go func(h *sda.Conode, m monitor.Measure) {
			ready <- true
			defer wg.Done()
			h.Start()
			// record bandwidth
			m.Record()
			log.Lvl3(hostAddress, "Simulation closed host", h.ServerIdentity)
		}(host, measures[i])
		// wait to be sure the goroutine started
		<-ready

		sim, err := sda.NewSimulation(simul, sc.Config)
		if err != nil {
			log.Fatal(err)
		}
		err = sim.Node(sc)
		if err != nil {
			log.Fatal(err)
		}
		sims[i] = sim
		if host.ServerIdentity.ID == sc.Tree.Root.ServerIdentity.ID {
			log.Lvl2(hostAddress, "is root-node, will start protocol")
			rootSim = sim
			rootSC = sc
		}
	}
	if rootSim != nil {
		// If this cothority has the root-host, it will start the simulation
		log.Lvl2("Starting protocol", simul, "on host", rootSC.Conode.ServerIdentity.Address)
		//log.Lvl5("Tree is", rootSC.Tree.Dump())

		// First count the number of available children
		childrenWait := monitor.NewTimeMeasure("ChildrenWait")
		wait := true
		// The timeout starts with 1 second, which is the time of response between
		// each level of the tree.
		timeout := 1000
		for wait {
			p, err := rootSC.Overlay.CreateProtocolSDA("Count", rootSC.Tree)
			if err != nil {
				log.Fatal(err)
			}
			proto := p.(*manage.ProtocolCount)
			proto.SetTimeout(timeout)
			proto.Start()
			log.Lvl1("Started counting children with timeout of", timeout)
			select {
			case count := <-proto.Count:
				if count == rootSC.Tree.Size() {
					log.Lvl1("Found all", count, "children")
					wait = false
				} else {
					log.Lvl1("Found only", count, "children, counting again")
				}
			}
			// Double the timeout and try again if not successful.
			timeout *= 2
		}
		childrenWait.Record()
		log.Lvl1("Starting new node", simul)
		measureNet := monitor.NewCounterIOMeasure("bandwidth_root", rootSC.Conode)
		err := rootSim.Run(rootSC)
		if err != nil {
			log.Fatal(err)
		}
		measureNet.Record()

		// Test if all ServerIdentities are used in the tree, else we'll run into
		// troubles with CloseAll
		if !rootSC.Tree.UsesList() {
			log.Error("The tree doesn't use all ServerIdentities from the list!\n" +
				"This means that the CloseAll will fail and the experiment never ends!")
		}
		closeTree := rootSC.Tree
		if rootSC.GetSingleHost() {
			// In case of "SingleHost" we need a new tree that contains every
			// entity only once, whereas rootSC.Tree will have the same
			// entity at different TreeNodes, which makes it difficult to
			// correctly close everything.
			log.Lvl2("Making new root-tree for SingleHost config")
			closeTree = rootSC.Roster.GenerateBinaryTree()
			rootSC.Overlay.RegisterTree(closeTree)
		}
		pi, err := rootSC.Overlay.CreateProtocolSDA("CloseAll", closeTree)
		pi.Start()
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Lvl3(hostAddress, scs[0].Conode.ServerIdentity, "is waiting for all hosts to close")
	wg.Wait()
	log.Lvl2(hostAddress, "has all hosts closed")
	monitor.EndAndCleanup()
}
