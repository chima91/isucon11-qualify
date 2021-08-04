package model

import (
	"crypto/md5"
	"io/ioutil"
	"log"
	"sync"
	"time"

	"github.com/isucon/isucon11-qualify/bench/service"

	"github.com/google/uuid"
	"github.com/isucon/isucon11-qualify/bench/random"
)

//enum
type IsuStateChange int

const (
	IsuStateChangeNone IsuStateChange = iota
	IsuStateChangeBad
	IsuStateChangeClear            = 1 << 3
	IsuStateChangeDetectOverweight = 1 << 4
	IsuStateChangeRepair           = 1 << 5
)

//poster Goroutineとシナリオ Goroutineとの通信に必要な情報
//ISU協会はこれを使ってposter Goroutineを起動、poster Goroutineはこれを使って通信
//複数回poster Goroutineが起動するかもしれないのでcloseしない
//当然リソースリークするがベンチマーカーは毎回落とすので問題ない
type StreamsForPoster struct {
	StateChan <-chan IsuStateChange
	//ConditionChan chan<- []IsuCondition
}

//poster Goroutineとシナリオ Goroutineとの通信に必要な情報
//複数回poster Goroutineが起動するかもしれないのでcloseしない
//当然リソースリークするがベンチマーカーは毎回落とすので問題ない
type StreamsForScenario struct {
	StateChan chan<- IsuStateChange
	//ConditionChan <-chan []IsuCondition
}

//一つのIsuにつき、一つの送信用 Goroutineがある
//IsuはISU協会 Goroutineからも読み込まれる
type Isu struct {
	Owner                         *User               `json:"-"`
	ID                            int                 `json:"id"`
	JIAIsuUUID                    string              `json:"jia_isu_uuid"`
	Name                          string              `json:"name"`
	ImageHash                     [md5.Size]byte      `json:"image_file_hash"` // 画像の検証用
	Character                     string              `json:"character"`
	StreamsForScenario            *StreamsForScenario `json:"-"`          //poster Goroutineとの通信
	Conditions                    IsuConditionArray   `json:"conditions"` //シナリオ Goroutineからのみ参照
	CondMutex                     sync.RWMutex
	LastCompletedGraphTime        int64     //シナリオ Goroutineからのみ参照
	PostTime                      time.Time //POST /isu/:id を叩いた仮想時間
	LastReadConditionTimestamp    int64     //シナリオ Goroutineからのみ参照
	LastReadBadConditionTimestamp int64     //シナリオ Goroutineからのみ参照
	CreatedAt                     time.Time `json:"created_at"`
}

//新しいISUの生成
//scenarioのNewIsu以外からは呼び出さないこと！
//戻り値を使ってbackendにpostする必要あり
//戻り値をISU協会にIsu*を登録する必要あり
//戻り値をownerに追加する必要あり
func NewRandomIsuRaw(owner *User) (*Isu, *StreamsForPoster, error) {
	stateChan := make(chan IsuStateChange, 1)
	//conditionChan := make(chan []IsuCondition, 10)

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, nil, err
	}
	isu := &Isu{
		Owner:      owner,
		JIAIsuUUID: id.String(),
		Name:       random.IsuName(),
		ImageHash:  defaultIconHash,
		Character:  random.Character(),
		StreamsForScenario: &StreamsForScenario{
			StateChan: stateChan,
			//ConditionChan: conditionChan,
		},
		Conditions: NewIsuConditionArray(),
	}

	streamsForPoster := &StreamsForPoster{
		StateChan: stateChan,
		//ConditionChan: conditionChan,
	}
	return isu, streamsForPoster, nil
}

func NewIsuRawForInitData(isu *Isu, owner *User, jiaIsuUUID string) (*Isu, *StreamsForPoster, error) {
	stateChan := make(chan IsuStateChange, 1)
	//conditionChan := make(chan []IsuCondition, 10)

	isu = &Isu{
		ID:         isu.ID,
		Owner:      owner,
		JIAIsuUUID: jiaIsuUUID,
		Name:       isu.Name,
		ImageHash:  isu.ImageHash,
		Character:  isu.Character,
		Conditions: isu.Conditions,
		StreamsForScenario: &StreamsForScenario{
			StateChan: stateChan,
			//ConditionChan: conditionChan,
		},
	}

	streamsForPoster := &StreamsForPoster{
		StateChan: stateChan,
		//ConditionChan: conditionChan,
	}

	return isu, streamsForPoster, nil
}

var defaultIconHash [md5.Size]byte

const defaultIconFilePath = "./images/default.jpg"

func init() {
	image, err := ioutil.ReadFile(defaultIconFilePath)
	if err != nil {
		log.Fatalf("failed to read default icon: %v", err)
	}
	defaultIconHash = md5.Sum(image)
}

func (isu *Isu) ToService() *service.Isu {
	return &service.Isu{
		ID:         isu.ID,
		JIAIsuUUID: isu.JIAIsuUUID,
		Name:       isu.Name,
		Character:  isu.Character,
	}
}

func (isu *Isu) SetImage(image []byte) {
	isu.ImageHash = md5.Sum(image)
}

func (i *Isu) AddIsuConditions(conditions []IsuCondition) {
	i.CondMutex.Lock()
	defer i.CondMutex.Unlock()
	for _, c := range conditions {
		i.Conditions.Add(&c)
	}
}

// func (i *Isu) GetConditions() []*IsuCondition {
// 	i.condMutex.RLock()
// 	defer i.condMutex.RUnlock()
// 	return i.conditions[:]
// }
