package conv

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"time"
)

func StringToInt64(s string) (int64, error) {
	newData, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}

	return newData, nil
}

func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371
	latDelta := (lat2 - lat1) * (math.Pi / 180)
	lonDelta := (lon2 - lon1) * (math.Pi / 180)
	a := math.Sin(latDelta/2)*math.Sin(latDelta/2) +
		math.Cos(lat1*(math.Pi/180))*math.Cos(lat2*(math.Pi/180))*
			math.Sin(lonDelta/2)*math.Sin(lonDelta/2)
	return earthRadiusKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func Int64PointerToInt64(num *int64) int64 {
	if num != nil {
		return *num
	}
	return 0
}

func GenerateOrderCode() string {
	return fmt.Sprintf("ORD-%s-%d", time.Now().Format("20060102150405"), rand.Intn(1000000))
}
