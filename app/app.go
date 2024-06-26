package app

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sync/errgroup"

	v3 "dxlib/v3"
	"dxlib/v3/api"
	"dxlib/v3/configurations"
	"dxlib/v3/core"
	"dxlib/v3/databases"
	"dxlib/v3/log"
	"dxlib/v3/redis"
	"dxlib/v3/tables"
	"dxlib/v3/tasks"
)

type DXAppArgCommandFunc func(s *DXApp, ac *DXAppArgCommand, T any) (err error)

type DXAppArgCommand struct {
	name     string
	command  string
	callback *DXAppArgCommandFunc
}

type DXAppArgOptionFunc func(s *DXApp, ac *DXAppArgOption, T any) (err error)

type DXAppArgOption struct {
	name     string
	option   string
	callback *DXAppArgOptionFunc
}

type DXAppArgs struct {
	Commands map[string]*DXAppArgCommand
	Options  map[string]*DXAppArgOption
}

type DXAppCallbackFunc func() (err error)
type DXAppEvent func() (err error)

type DXApp struct {
	nameId                   string
	Title                    string
	Description              string
	Version                  string
	Args                     DXAppArgs
	IsLoop                   bool
	RuntimeErrorGroup        *errgroup.Group
	RuntimeErrorGroupContext context.Context

	IsRedisExist          bool
	IsStorageExist        bool
	IsAPIExist            bool
	IsTaskExist           bool
	DebugKey              string
	IsDebug               bool
	OnDefine              DXAppEvent
	OnDefineConfiguration DXAppEvent
	OnDefineAPI           DXAppEvent
	OnExecute             DXAppEvent
	OnStartStorageReady   DXAppEvent
	OnStopping            DXAppEvent
}

func (a *DXApp) Run() error {
	if a.OnDefine != nil {
		err := a.OnDefine()
		if err != nil {
			log.Log.Error(err.Error())
			return err
		}
	}
	if a.OnDefineConfiguration != nil {
		err := a.OnDefineConfiguration()
		if err != nil {
			log.Log.Error(err.Error())
			return err
		}
	}
	if a.OnDefineAPI != nil {
		err := a.OnDefineAPI()
		if err != nil {
			log.Log.Error(err.Error())
			return err
		}
	}

	err := a.execute()
	if err != nil {
		log.Log.Error(err.Error())
		return err
	}
	return nil
}

func (a *DXApp) start() (err error) {
	log.Log.Info(fmt.Sprintf("%v %v %v", a.Title, a.Version, a.Description))
	err = configurations.Manager.Load()
	if err != nil {
		return err
	}
	_, a.IsRedisExist = configurations.Manager.Configurations["redis"]
	if a.IsRedisExist {
		err = redis.Manager.LoadFromConfiguration("redis")
		if err != nil {
			return err
		}
	}
	_, a.IsStorageExist = configurations.Manager.Configurations["storage"]
	if a.IsStorageExist {
		err = databases.Manager.LoadFromConfiguration("storage")
		if err != nil {
			return err
		}
	}
	_, a.IsAPIExist = configurations.Manager.Configurations["api"]

	if a.IsRedisExist {
		err = redis.Manager.ConnectAllAtStart()
		if err != nil {
			return err
		}
	}
	if a.IsStorageExist {
		err = databases.Manager.ConnectAllAtStart(`storage`)
		if err != nil {
			return err
		}
		err := tables.Manager.ConnectAll()
		if err != nil {
			return err
		}
		if a.OnStartStorageReady != nil {
			err = a.OnStartStorageReady()
			if err != nil {
				return err
			}

		}
	}
	if a.IsAPIExist {
		err = api.Manager.StartAll(a.RuntimeErrorGroup, a.RuntimeErrorGroupContext)
		if err != nil {
			return err
		}
	}
	_, a.IsTaskExist = configurations.Manager.Configurations["tasks"]

	if a.IsTaskExist {
		err = tasks.Manager.StartAll(a.RuntimeErrorGroup, a.RuntimeErrorGroupContext)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *DXApp) Stop() (err error) {
	log.Log.Info("Stopping")
	if a.OnStopping != nil {
		a.OnStopping()
	}
	if a.IsTaskExist {
		err = tasks.Manager.StopAll()
		if err != nil {
			return err
		}
	}
	if a.IsAPIExist {
		err = api.Manager.StopAll()
		if err != nil {
			return err
		}
	}
	if a.IsRedisExist {
		err = redis.Manager.DisconnectAll()
		if err != nil {
			return err
		}
	}
	if a.IsStorageExist {
		err = databases.Manager.DisconnectAll()
		if err != nil {
			return err
		}
	}
	log.Log.Info("Stopped")
	return nil
}

func (a *DXApp) execute() (err error) {
	defer core.RootContextCancel()
	a.RuntimeErrorGroup, a.RuntimeErrorGroupContext = errgroup.WithContext(core.RootContext)
	err = a.start()
	if err != nil {
		return err
	}
	if a.IsLoop {
		defer func() {
			err2 := a.Stop()
			if err2 != nil {
				log.Log.Infof("Error in Stopping: (%v)", err2)
			}
			//log.Log.Info("Stopped")
		}()
	}
	log.Log.Info("Starting")

	if a.OnExecute != nil {
		err = a.OnExecute()
		if err != nil {
			log.Log.Infof("onExecute error (%v)", err)
			return err
		}
	}

	if a.IsLoop {
		log.Log.Info("Waiting...")
		err = a.RuntimeErrorGroup.Wait()
		if err != nil {
			log.Log.Infof("Exit reason: %v", err)
			return err
		}
	}
	return nil
}

var App DXApp

func Set(nameId, title, description string, isLoop bool, debugKey string) {
	v3.AppNameId = nameId
	App.nameId = nameId
	App.Title = title
	App.Description = description
	App.IsLoop = isLoop
	App.DebugKey = debugKey
	App.IsDebug = os.Getenv("DEBUG_KEY") == debugKey
	log.Log.Prefix = nameId
}

func GetNameId() string {
	return App.nameId
}
func init() {
	App = DXApp{
		Args: DXAppArgs{
			Commands: map[string]*DXAppArgCommand{},
			Options:  map[string]*DXAppArgOption{},
		},
		IsDebug: false,
	}
}
