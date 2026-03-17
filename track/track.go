package track

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// Location 对应 map.json 中的节点结构
type Location struct {
	ID       int    `json:"id"`
	Location string `json:"location"` // 格式: "lng,lat"
	Edge     []int  `json:"edge"`
}

// Gen 生成轨迹
func Gen(distance int64, locations []Location) string {
	var currentDistance float64 = 0

	// 初始化随机种子
	rand.Seed(time.Now().UnixNano())

	// 随机起始点
	startIndex := rand.Intn(len(locations))
	currentLocation := locations[startIndex]

	var result []string

	// 时间推前30分钟 (毫秒)
	startTime := time.Now().UnixMilli() - 30*60*1000
	lastIndex := -1

	currentCoords := parseCoords(currentLocation.Location)
	result = append(result, fmt.Sprintf("%f-%f-%d-%.1f", currentCoords[0], currentCoords[1], startTime, randAccuracy()))

	for currentDistance < float64(distance) {
		currentCoords = parseCoords(currentLocation.Location)
		edges := currentLocation.Edge

		if len(edges) == 0 {
			fmt.Println("警告: 节点 edge 为空")
			break
		}

		randInt := rand.Intn(len(edges))
		edgeIndex := edges[randInt]

		// 尽量不往回走
		if edgeIndex == lastIndex {
			edgeIndex = edges[(randInt+1)%len(edges)]
		}

		// 防御性检查，避免 map.json 异常导致越界
		if edgeIndex < 0 || edgeIndex >= len(locations) {
			fmt.Println("警告: edge 索引越界")
			break
		}

		next := locations[edgeIndex]
		endCoords := parseCoords(next.Location)

		goDistance := CalculateDistance(currentCoords, endCoords)
		currentDistance += goDistance

		lastRandPos := currentCoords
		for i := 0; i < 10; i++ {
			newRandPos := randPos(lastRandPos, endCoords)
			dist := CalculateDistance(lastRandPos, newRandPos)
			lastRandPos = newRandPos

			// 随机速度 1-4 m/s (规避除数为0)
			speed := rand.Intn(4) + 1
			startTime += int64((dist / float64(speed)) * 1000)
			result = append(result, fmt.Sprintf("%f-%f-%d-%.1f", lastRandPos[0], lastRandPos[1], startTime, randAccuracy()))
		}

		dist := CalculateDistance(lastRandPos, endCoords)
		speed := rand.Intn(4) + 1
		startTime += int64((dist / float64(speed)) * 1000)
		result = append(result, fmt.Sprintf("%f-%f-%d-%.1f", endCoords[0], endCoords[1], startTime, randAccuracy()))

		lastIndex = currentLocation.ID
		currentLocation = next
	}

	startTime += int64((rand.Intn(6) + 5) * 1000) // 最后停留 5-10秒
	replace := strings.ReplaceAll(currentLocation.Location, ",", "-")
	result = append(result, fmt.Sprintf("%s-%d-%.1f", replace, startTime, randAccuracy()))

	// 将 result 数组转为 JSON 字符串格式（类似 Java 的 JsonUtils.obj2String）
	return fmt.Sprintf("[\"%s\"]", strings.Join(result, "\",\""))
}

// 辅助函数：解析 "lng,lat" 为 float64 数组
func parseCoords(loc string) []float64 {
	parts := strings.Split(loc, ",")
	lng, _ := strconv.ParseFloat(parts[0], 64)
	lat, _ := strconv.ParseFloat(parts[1], 64)
	return []float64{lng, lat}
}

// 辅助函数：计算两点距离 (Haversine formula)
func CalculateDistance(start, end []float64) float64 {
	const R = 6371000 // 地球半径(米)
	lat1 := start[1] * math.Pi / 180
	lat2 := end[1] * math.Pi / 180
	deltaLat := (end[1] - start[1]) * math.Pi / 180
	deltaLng := (end[0] - start[0]) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(deltaLng/2)*math.Sin(deltaLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// 辅助函数：线段上的随机点
func randPos(start, end []float64) []float64 {
	r := rand.Float64()
	dx := end[0] - start[0]
	dy := end[1] - start[1]
	return []float64{start[0] + dx*r, start[1] + dy*r}
}

// 辅助函数：随机精度
func randAccuracy() float64 {
	return 10 * rand.Float64()
}
