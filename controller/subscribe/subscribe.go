package subscribe

import (
	"strconv"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eric2788/biligo-live-ws/services/api"
	"github.com/eric2788/biligo-live-ws/services/subscriber"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	log = logrus.WithField("controller", "subscribe")
)

func Register(gp *gin.RouterGroup) {
	gp.GET("", GetSubscriptions)
	gp.POST("", Subscribe)
	gp.DELETE("", ClearSubscribe)
	gp.PUT("add", AddSubscribe)
	gp.PUT("remove", RemoveSubscribe)
}

func GetSubscriptions(c *gin.Context) {
	list, ok := subscriber.Get(c.GetString("identifier"))
	if !ok {
		list = []int64{}
	}
	c.IndentedJSON(200, list)
}

func ClearSubscribe(c *gin.Context) {
	subscriber.Delete(c.GetString("identifier"))
	c.Status(200)
}

func AddSubscribe(c *gin.Context) {

	dontCheck := c.Query("validate") == "false" // 是否不检查房间讯息
	rooms, ok := GetSubscribesArr(c, !dontCheck)

	if !ok {
		return
	}

	log.Infof("用戶 %v 新增訂閱 %v \n", c.GetString("identifier"), rooms)
	newRooms := subscriber.Add(c.GetString("identifier"), rooms)
	go ActivateExpire(c.GetString("identifier"))
	c.IndentedJSON(200, newRooms)
}

func RemoveSubscribe(c *gin.Context) {

	rooms, ok := GetSubscribesArr(c, false) // 刪除訂閱不檢查房間訊息是否存在

	if !ok {
		return
	}

	log.Infof("用戶 %v 移除訂閱 %v \n", c.GetString("identifier"), rooms)

	newRooms, ok := subscriber.Remove(c.GetString("identifier"), rooms)

	if !ok {
		c.IndentedJSON(400, gin.H{"error": "刪除失敗，你尚未遞交過任何訂閱"})
		return
	}

	c.IndentedJSON(200, newRooms)
}

func Subscribe(c *gin.Context) {
	dontCheck := c.Query("validate") == "false" // 是否不检查房间讯息
	rooms, ok := GetSubscribesArr(c, !dontCheck)

	if !ok {
		return
	}

	log.Infof("用戶 %v 設置訂閱 %v \n", c.GetString("identifier"), rooms)

	subscriber.Update(c.GetString("identifier"), rooms)
	go ActivateExpire(c.GetString("identifier"))
	c.IndentedJSON(200, rooms)
}

func GetSubscribesArr(c *gin.Context, checkExist bool) ([]int64, bool) {

	subArr, ok := c.GetPostFormArray("subscribes")
	if !ok {
		c.AbortWithStatusJSON(400, gin.H{"error": "缺少 `subscribes` 數值(訂閱列表)"})
		return nil, false
	}
	if len(subArr) == 0 {
		c.AbortWithStatusJSON(400, gin.H{"error": "訂閱列表不能為空"})
		return nil, false
	}

	roomSet := mapset.NewSet[int64]()

	for _, arr := range subArr {

		roomId, err := strconv.ParseInt(arr, 10, 64)

		if err != nil {
			log.Warn("cannot parse room: ", err.Error())
			continue
		}

		if checkExist {

			realRoom, roomErr := api.GetRealRoom(roomId)

			if roomErr != nil {
				log.Warnf("獲取房間訊息時出現錯誤: %v", roomErr)
				_ = c.Error(roomErr)
				return nil, false
			} else {
				if realRoom > 0 {
					roomSet.Add(realRoom)
				} else {
					log.Warnf("房間 %v 無效，已略過 \n", roomId)
				}
			}
		} else {
			roomSet.Add(roomId)
		}

	}
	return roomSet.ToSlice(), true
}

func ActivateExpire(identifier string) {
	// 設置如果五分鐘後尚未連線 WebSocket 就清除訂閱記憶
	subscriber.ExpireAfter(identifier, time.NewTimer(time.Minute*5))
}
