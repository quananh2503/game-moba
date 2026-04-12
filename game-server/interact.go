package main

import (
	def "game/pkg"
	"math"
)

// ---------------------------------------------------------
// TRỢ THỦ 1: TRÒN vs TRÒN
// ---------------------------------------------------------
func checkCircleVsCircle(x1, y1, r1, x2, y2, r2 float32) bool {
	dx := x1 - x2
	dy := y1 - y2
	radSum := r1 + r2
	return (dx*dx + dy*dy) <= (radSum * radSum)
}

// ---------------------------------------------------------
// TRỢ THỦ 2: HỘP THẲNG (AABB) vs HỘP THẲNG (AABB)
// ---------------------------------------------------------
func checkAABBVsAABB(x1, y1, w1, h1, x2, y2, w2, h2 float32) bool {
	return (math.Abs(float64(x1-x2)) * 2 < float64(w1+w2)) &&
	       (math.Abs(float64(y1-y2)) * 2 < float64(h1+h2))
}

// ---------------------------------------------------------
// TRỢ THỦ 3: TRÒN vs HỘP THẲNG (AABB)
// ---------------------------------------------------------
func checkCircleVsAABB(cx, cy, r float32, boxX, boxY, w, h float32) bool {
	closestX := float32(math.Max(float64(boxX-w/2), math.Min(float64(cx), float64(boxX+w/2))))
	closestY := float32(math.Max(float64(boxY-h/2), math.Min(float64(cy), float64(boxY+h/2))))
	dx := cx - closestX
	dy := cy - closestY
	return (dx*dx + dy*dy) <= (r * r)
}

// ---------------------------------------------------------
// TRỢ THỦ 4: TRÒN vs HỘP XOAY CHÉO (OBB) - KIỆT TÁC TOÁN HỌC
// ---------------------------------------------------------
func checkCircleVsOBB(cx, cy, r float32, rectX, rectY, w, h float32, angleDeg uint16) bool {
	dx := cx - rectX
	dy := cy - rectY

	rad := -float64(angleDeg) * math.Pi / 180.0
	cosA := float32(math.Cos(rad))
	sinA := float32(math.Sin(rad))

	localX := dx*cosA - dy*sinA
	localY := dx*sinA + dy*cosA

	halfW := w / 2.0
	halfH := h / 2.0

	closestX := float32(math.Max(float64(-halfW), math.Min(float64(localX), float64(halfW))))
	closestY := float32(math.Max(float64(-halfH), math.Min(float64(localY), float64(halfH))))

	distX := localX - closestX
	distY := localY - closestY

	forgivenessRadius := r * 1.05 // Cộng 5% nịnh người chơi

	return (distX*distX + distY*distY) <= (forgivenessRadius * forgivenessRadius)
}
// LƯU Ý: Đã thêm angle1 và angle2 vào tham số
func checkInteract2Collider(x1, y1 float32, c1 Collider, angle1 uint16, x2, y2 float32, c2 Collider, angle2 uint16) bool {

	// ==========================================
	// 1. NGƯỜI CHƠI vs NGƯỜI CHƠI / ĐẠN vs NGƯỜI CHƠI (Tròn vs Tròn)
	// ==========================================
	if c1.ShapeType == def.ShapeCircle && c2.ShapeType == def.ShapeCircle {
		return checkCircleVsCircle(x1, y1, c1.Radius, x2, y2, c2.Radius)
	}

	// ==========================================
	// 2. NGƯỜI CHƠI vs TƯỜNG BẢN ĐỒ (Tròn vs Box thẳng)
	// ==========================================
	if c1.ShapeType == def.ShapeCircle && c2.ShapeType == def.ShapeBox {
		return checkCircleVsAABB(x1, y1, c1.Radius, x2, y2, c2.Width, c2.Height)
	}
	if c1.ShapeType == def.ShapeBox && c2.ShapeType == def.ShapeCircle {
		return checkCircleVsAABB(x2, y2, c2.Radius, x1, y1, c1.Width, c1.Height)
	}

	// ==========================================
	// 3. NGƯỜI CHƠI vs CHIÊU THỨC (Tròn vs OBB xoay chéo)
	// ==========================================
	if c1.ShapeType == def.ShapeCircle && c2.ShapeType == def.ShapeOBB {
		// Góc truyền vào phải là góc của OBB (angle2)
		return checkCircleVsOBB(x1, y1, c1.Radius, x2, y2, c2.Width, c2.Height, angle2)
	}
	if c1.ShapeType == def.ShapeOBB && c2.ShapeType == def.ShapeCircle {
		// Chiêu thức quét người chơi -> góc của chiêu thức là angle1
		return checkCircleVsOBB(x2, y2, c2.Radius, x1, y1, c1.Width, c1.Height, angle1)
	}

	// ==========================================
	// 4. (Tuỳ chọn) TƯỜNG ĐỘNG vs TƯỜNG TĨNH (Box vs Box)
	// ==========================================
	if c1.ShapeType == def.ShapeBox && c2.ShapeType == def.ShapeBox {
		return checkAABBVsAABB(x1, y1, c1.Width, c1.Height, x2, y2, c2.Width, c2.Height)
	}

	// Nếu rơi vào các trường hợp không cần thiết (VD: OBB vs OBB), tự động bỏ qua.
	return false 
}