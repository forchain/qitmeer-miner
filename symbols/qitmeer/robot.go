// Copyright (c) 2019 The qitmeer developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.
package qitmeer

import (
	"fmt"
	"github.com/Qitmeer/go-opencl/cl"
	"log"
	"qitmeer-miner/common"
	"qitmeer-miner/core"
	"qitmeer-miner/stats_server"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	POW_DOUBLE_BLAKE2B = "blake2bd"
	POW_CUCKROO = "cuckaroo"
	POW_CUCKROO29 = "cuckaroo29"
	POW_CUCKTOO = "cuckatoo"
)
type QitmeerRobot struct {
	core.MinerRobot
	Work                 QitmeerWork
	Devices              []core.BaseDevice
	Stratu               *QitmeerStratum
	AllTransactionsCount int64
}

func (this *QitmeerRobot)GetPow(i int ,device *cl.Device) core.BaseDevice{
	switch this.Cfg.NecessaryConfig.Pow {
	case POW_CUCKROO:
		deviceMiner := &Cuckaroo{}
		deviceMiner.MiningType = "cuckaroo"
		deviceMiner.Init(i,device,this.Pool,this.Quit,this.Cfg)
		this.Devices = append(this.Devices,deviceMiner)
		return deviceMiner
	case POW_CUCKTOO:
		deviceMiner := &Cuckatoo{}
		deviceMiner.MiningType = "cuckatoo"
		deviceMiner.Init(i,device,this.Pool,this.Quit,this.Cfg)
		this.Devices = append(this.Devices,deviceMiner)
		return deviceMiner
	case POW_DOUBLE_BLAKE2B:
		deviceMiner := &Blake2bD{}
		deviceMiner.MiningType = "blake2bd"
		deviceMiner.Init(i,device,this.Pool,this.Quit,this.Cfg)
		this.Devices = append(this.Devices,deviceMiner)
		return deviceMiner

	default:
		log.Fatalln(this.Cfg.NecessaryConfig.Pow," pow has not exist!")
	}
	return nil
}

func (this *QitmeerRobot)InitDevice()  {
	this.MinerRobot.InitDevice()
	for i, device := range this.ClDevices {
		deviceMiner := this.GetPow(i ,device)
		if deviceMiner == nil{
			return
		}
	}
}

// runing
func (this *QitmeerRobot)Run() {
	this.Wg = &sync.WaitGroup{}
	this.InitDevice()
	//mining service
	connectName := "solo"
	this.Pool = false
	if this.Cfg.PoolConfig.Pool != ""{ //is pool mode
		connectName = "pool"
		this.Stratu = &QitmeerStratum{}
		_ = this.Stratu.StratumConn(this.Cfg)
		this.Wg.Add(1)
		go func() {
			defer this.Wg.Done()
			this.Stratu.HandleReply()
		}()
		this.Pool = true
	}
	common.MinerLoger.Infof("%s miner start",connectName)
	this.Work = QitmeerWork{}
	this.Work.Cfg = this.Cfg
	this.Work.Rpc = this.Rpc
	this.Work.stra = this.Stratu
	// Device Miner
	for _,dev := range this.Devices{
		dev.SetIsValid(true)
		dev.InitDevice()
		dev.SetPool(this.Pool)
		if len(this.UseDevices) > 0 && !common.InArray(strconv.Itoa(dev.GetMinerId()),this.UseDevices){
			dev.SetIsValid(false)
		}
		this.Wg.Add(1)
		go dev.Mine(this.Wg)
		this.Wg.Add(1)
		go dev.Status(this.Wg)
	}
	//refresh work
	this.Wg.Add(1)
	go func(){
		defer this.Wg.Done()
		this.ListenWork()
	}()
	//submit work
	this.Wg.Add(1)
	go func(){
		defer this.Wg.Done()
		this.SubmitWork()
	}()
	//submit status
	this.Wg.Add(1)
	go func(){
		defer this.Wg.Done()
		this.Status()
	}()

	//http server stats
	if this.Cfg.OptionConfig.StatsServer != ""{
		this.Wg.Add(1)
		go func(){
			defer this.Wg.Done()
			stats_server.HandleRouter(this.Cfg,this.Devices)
		}()
	}

	this.Wg.Wait()
}

// ListenWork
func (this *QitmeerRobot)ListenWork() {
	common.MinerLoger.Info("listen new work server")
	t := time.NewTicker(time.Second * 2)
	defer t.Stop()
	r := false
	for {
		select {
		case <-this.Quit:
			return
		case <-t.C:
			r = false
			if this.Pool {
				r = this.Work.PoolGet() // get new work
			} else {
				r = this.Work.Get() // get new work
			}
			if r {
				for _, dev := range this.Devices {
					if !dev.GetIsValid(){
						continue
					}
					dev.SetNewWork(&this.Work)
				}
			}
		}
	}
}

// ListenWork
func (this *QitmeerRobot)SubmitWork() {
	common.MinerLoger.Info("listen submit block server")
	go func() {
		str := ""
		var logContent string
		var count int
		var arr []string
		for{
			select {
			case <-this.Quit:
				return
			case str = <-this.SubmitStr:
				if str == ""{
					atomic.AddUint64(&this.StaleShares, 1)
					continue
				}
				var err error
				var height ,txCount ,block string
				if this.Pool {
					arr = strings.Split(str,"-")
					block = arr[0]
					err = this.Work.PoolSubmit(str)
				} else {
					//solo miner
					arr = strings.Split(str,"-")
					txCount = arr[1]
					height = arr[2]
					block = arr[0]
					err = this.Work.Submit(block)
				}
				if err != nil{
					if err != ErrSameWork || err == ErrSameWork{
						if err == ErrStratumStaleWork{
							atomic.AddUint64(&this.StaleShares, 1)
						} else{
							atomic.AddUint64(&this.InvalidShares, 1)
						}
					}
				} else {
					atomic.AddUint64(&this.ValidShares, 1)
					if !this.Pool{
						count ,_ = strconv.Atoi(txCount)
						this.AllTransactionsCount += int64(count)
						logContent = fmt.Sprintf("receive block, block height = %s,Including %s transactions(not contain coinbase tx); Received Total transactions = %d\n",
							height,txCount,this.AllTransactionsCount)
						common.MinerLoger.Info(logContent)
					}
				}
			}
		}

	}()
	for _,dev := range this.Devices{
		go dev.SubmitShare(this.SubmitStr)
	}
}

// stats the submit result
func (this *QitmeerRobot)Status()  {
	t := time.NewTicker(time.Second * 30)
	var valid,rejected,staleShares uint64
	defer t.Stop()
	for {
		select {
		case <-this.Quit:
			return
		case <-t.C:
			if this.Work.stra == nil && this.Work.Block.Height == 0{
				continue
			}
			valid = atomic.LoadUint64(&this.ValidShares)
			rejected = atomic.LoadUint64(&this.InvalidShares)
			staleShares = atomic.LoadUint64(&this.StaleShares)
			if this.Pool{
				valid = atomic.LoadUint64(&this.Stratu.ValidShares)
				rejected = atomic.LoadUint64(&this.Stratu.InvalidShares)
				staleShares = atomic.LoadUint64(&this.Stratu.StaleShares)
			}
			this.Cfg.OptionConfig.Accept = int(valid)
			this.Cfg.OptionConfig.Reject = int(rejected)
			this.Cfg.OptionConfig.Stale = int(staleShares)
			total := valid + rejected + staleShares
			common.MinerLoger.Infof("Global stats: Accepted: %v,Stale: %v, Rejected: %v, Total: %v",
				valid,
				staleShares,
				rejected,
				total,
			)
		}
	}
}