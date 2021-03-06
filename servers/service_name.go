package servers

import (
	"errors"
	"fmt"
	"github.com/gargous/flitter/common"
	"github.com/gargous/flitter/core"
	//saver "github.com/gargous/flitter/save"
	socketio "github.com/googollee/go-socket.io"
	"strings"
	"sync"
)

type NameService interface {
	Service
}
type namesrv struct {
	referee   Referee
	nameTrees map[string]core.NodeTree
	mutex     sync.Mutex
	bussyness bool
	baseService
}

func NewNameService() NameService {
	srv := &namesrv{
		nameTrees: make(map[string]core.NodeTree),
		bussyness: false,
	}
	srv.looper = core.NewMessageLooper(__LooperSize)
	return srv
}

func (n *namesrv) Init(srv interface{}) error {
	n.referee = srv.(Referee)
	n.HandleMessages()
	n.HandleClients()
	return nil
}
func (n *namesrv) SearchNodeInfo(npath core.NodePath) (opath core.NodePath, err error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	treeName, ok := npath.GetGroupName()
	if !ok || treeName == "" {
		err = errors.New("Group Name Not Exsit")
		return
	}
	tree, ok := n.nameTrees[treeName]
	if !ok {
		tree = core.NewNodeTree()
	}
	ninfo, ok := npath.GetNodeInfo()
	if !ok {
		err = errors.New("Invalid NodeInfo")
		return
	}
	opath, ok = tree.Search(ninfo)
	if !ok {
		opath, err = tree.Add(npath)
		if err != nil {
			return
		}
	}
	n.nameTrees[treeName] = tree
	return
}
func (n *namesrv) SearchNodeInfoWithGroupName(groupname string, index int) core.NodePath {
	tree, ok := n.nameTrees[groupname]
	if !ok {
		return ""
	}
	_index := 0
	targetAddress := tree.FLoopGroup(groupname, func(height int, node core.NodeInfo) bool {
		if _index == index {
			return true
		}
		_index++
		return false
	})
	return targetAddress
}
func (n *namesrv) HandleClients() {
	n.referee.OnClient("flitter refer address", func(so socketio.Socket) interface{} {
		return func(name string, index int) {
			if !n.bussyness {
				addr := n.SearchNodeInfoWithGroupName(name, index)
				so.Emit("flitter refer address", addr)
			} else {
				so.Emit("flitter refer address", __Client_Reply_bussy)
			}
		}
	})
}
func (n *namesrv) HandleMessages() {
	n.looper.AddHandler(0, core.MA_Refer, func(msg core.Message) (err error) {
		_, state, _ := msg.GetInfo().Info()
		switch state {
		case core.MS_Ask:
			if !n.bussyness {
				content, ok := msg.GetContent(0)
				if !ok {
					return
				}
				nodeinfo, err := n.SearchNodeInfo(core.NodePath(content))
				if err != nil {
					return err
				}
				msg.ClearContent()
				msg.AppendContent([]byte(nodeinfo))
				msg.GetInfo().SetState(core.MS_Succeed)
				err = n.referee.SendToWroker(msg, nodeinfo)
				if err != nil {
					return err
				}
			}
		case core.MS_Error:
			common.ErrIn(errors.New(msg.GetInfo().String()), "[node server]")
		}
		return nil
	})
}
func (n *namesrv) Start() {
	n.looper.Loop()
}
func (n *namesrv) Term() {
	n.looper.Term()
}
func (n namesrv) String() string {
	nameTressStr := ""
	for _, tree := range n.nameTrees {
		nameTressStr += strings.Join(strings.Split(fmt.Sprintf("%v", tree), "\n"), "\n\t")
		nameTressStr += "\n"
	}
	str := fmt.Sprintf("Name Service:["+
		"\n\tlooper:%p"+
		"\n\ttree:%v"+
		"\n]",
		n.looper,
		nameTressStr,
	)
	return str
}
