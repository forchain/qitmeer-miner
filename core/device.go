/**
Qitmeer
james
*/
package core

import (
	"github.com/Qitmeer/go-opencl/cl"
	"math"
	"os"
	"qitmeer-miner/common"
	"sync"
	"time"
)

type BaseDevice interface {
	Mine(wg *sync.WaitGroup)
	Update()
	InitDevice()
	Status(wg *sync.WaitGroup)
	GetIsValid() bool
	SetIsValid(valid bool)
	GetMinerId() int
	GetAverageHashRate() float64
	GetName() string
	GetStart() uint64
	GetIntensity() int
	GetWorkSize() int
	SetIntensity(inter int)
	SetWorkSize(lsize int)
	SetPool(pool bool)
	SetNewWork(w BaseWork)
	Release()
	GetMinerType() string
	SubmitShare(substr chan string)
}
type Device struct{
	Cfg *common.GlobalConfig  //must init
	DeviceName string
	HasNewWork bool
	AllDiffOneShares uint64
	AverageHashRate    float64
	MinerId          uint32
	Context          *cl.Context
	CommandQueue     *cl.CommandQueue
	LocalItemSize     int
	NonceOut     []byte
	BlockObj     *cl.MemObject
	NonceOutObj     *cl.MemObject
	NonceRandObj     *cl.MemObject
	Target2Obj     *cl.MemObject
	Kernel     *cl.Kernel
	Program     	*cl.Program
	ClDevice         *cl.Device
	Started          int64
	GlobalItemSize int
	CurrentWorkID uint64
	Quit chan os.Signal //must init
	sync.Mutex
	Wg         sync.WaitGroup
	Pool       bool //must init
	IsValid    bool //is valid
	SubmitData chan string //must
	NewWork    chan BaseWork
	Err        error
	MiningType        string
	Event *cl.Event
}

func (this *Device)Init(i int,device *cl.Device,pool bool,q chan os.Signal,cfg *common.GlobalConfig)  {
	this.MinerId = uint32(i)
	this.NewWork = make(chan BaseWork,1)
	this.Cfg=cfg
	this.DeviceName=device.Name()
	this.ClDevice=device
	this.CurrentWorkID=0
	this.IsValid=true
	this.Pool=pool
	this.SubmitData=make(chan string,1)
	this.GlobalItemSize= int(math.Exp2(float64(this.Cfg.OptionConfig.Intensity)))
	this.Quit=q
	this.AllDiffOneShares = 0
}

func (this *Device)GetIsValid() bool {
	return this.IsValid
}

func (this *Device)SetNewWork(work BaseWork) {
	this.HasNewWork = true
	this.NewWork <- work
}


func (this *Device)GetMinerType() string{
	return this.MiningType
}

func (this *Device)Update()  {
	this.CurrentWorkID,this.Err = common.RandUint64()
	if this.Err != nil{
		this.CurrentWorkID++
	}
}

func (this *Device)InitDevice()  {
	var err error
	this.Context, err = cl.CreateContext([]*cl.Device{this.ClDevice})
	if err != nil {
		this.IsValid = false
		common.MinerLoger.Infof("-%d %v CreateContext", this.MinerId, err)
		return
	}
	this.CommandQueue, err = this.Context.CreateCommandQueue(this.ClDevice, 0)
	if err != nil {
		this.IsValid = false
		common.MinerLoger.Infof("-%d %v CreateCommandQueue", this.MinerId,  err)
	}
}

func (this *Device)SetPool(b bool)  {
	this.Pool = b
}

func (this *Device)SetIsValid(valid bool) {
	this.IsValid = valid
}

func (this *Device)GetMinerId() int {
	return int(this.MinerId)
}

func (this *Device)GetIntensity() int {
	return int(math.Log2(float64(this.GlobalItemSize)))
}

func (this *Device)GetWorkSize() int {
	return this.LocalItemSize
}

func (this *Device)SetIntensity(inter int) {
	this.GlobalItemSize = int(math.Exp2(float64(this.Cfg.OptionConfig.Intensity)))
}

func (this *Device)SetWorkSize(size int) {
	this.LocalItemSize = size
}

func (this *Device)GetName() string {
	return this.DeviceName
}

func (this *Device)GetStart() uint64 {
	return uint64(this.Started)
}

func (this *Device)GetAverageHashRate() float64 {
	return this.AverageHashRate
}

func (d *Device)Release()  {
	d.Kernel.Release()
	d.Context.Release()
	d.BlockObj.Release()
	d.NonceOutObj.Release()
	d.Program.Release()
	d.NonceRandObj.Release()
	d.Target2Obj.Release()
	d.CommandQueue.Release()
}

func (this *Device)Status(wg *sync.WaitGroup)  {
	defer wg.Done()
	t := time.NewTicker(time.Second * 5)
	defer t.Stop()
	for {
		select{
		case <- this.Quit:
			return
		case <- t.C:
			if !this.IsValid{
				time.Sleep(2*time.Second)
				continue
			}
			secondsElapsed := time.Now().Unix() - this.Started
			//diffOneShareHashesAvg := uint64(0x00000000FFFFFFFF)
			if this.AllDiffOneShares <= 0 || secondsElapsed <= 0{
				continue
			}
			averageHashRate := float64(this.AllDiffOneShares) /
				float64(secondsElapsed)
			if this.AverageHashRate <= 0{
				this.AverageHashRate = averageHashRate
			}
			//recent stats 95% percent
			this.AverageHashRate = (this.AverageHashRate*50+averageHashRate*950)/1000
			unit := " H/s"
			if this.GetMinerType() != "blake2bd"{
				unit = " GPS"
			}
			common.MinerLoger.Infof("DEVICE_ID #%d (%s) %v",
				this.MinerId,
				this.ClDevice.Name(),
				common.FormatHashRate(this.AverageHashRate,unit),
			)
			// restats every 2min
			// Prevention this.AllDiffOneShares was to large
			if secondsElapsed > 120{
				this.Started = time.Now().Unix()
				this.AllDiffOneShares = 0
			}
		}
	}
}

func (this *Device) SubmitShare(substr chan string) {
	for {
		select {
		case <-this.Quit:
			return
		case str := <-this.SubmitData:
			if this.HasNewWork {
				//the stale submit
				continue
			}
			substr <- str
		}
	}
}
