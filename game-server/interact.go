package main

import (
	"fmt"
	def "game/pkg"
	"math"
)
var SinTable [360]float32
var CosTable [360]float32
func InitMathTables() {
	for i := 0; i < 360; i++ {
		// Chuyển Độ (Degree) sang Radian để tính toán gốc
		rad := float64(i) * math.Pi / 180.0
		// Tính sẵn và lưu dạng float32
		SinTable[i] = float32(math.Sin(rad))
		CosTable[i] = float32(math.Cos(rad))
	}
	fmt.Println("[System] Đã nạp xong Bảng tra cứu Sin/Cos (LUT).")
}
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
	halfW := w * 0.5 // Nhân 0.5 luôn nhanh hơn chia 2 (phép nhân tốn 1 cycle, chia tốn ~10 cycles)
    halfH := h * 0.5
	closestX := max(boxX-halfW, min(cx, boxX+halfW))
    closestY := max(boxY-halfH, min(cy, boxY+halfH))
	dx := cx - closestX
	dy := cy - closestY
	// fmt.Printf("dx %f dy %f -%f <=%f \n",dx,dy,(dx*dx + dy*dy),r * r)
	return (dx*dx + dy*dy) <= (r * r)
}

// ---------------------------------------------------------
// TRỢ THỦ 4: TRÒN vs HỘP XOAY CHÉO (OBB) - KIỆT TÁC TOÁN HỌC
// ---------------------------------------------------------
func checkCircleVsOBB(cx, cy, r float32, rectX, rectY, w, h float32, angleDeg uint16) bool {
	dx := cx - rectX
	dy := cy - rectY

	safeAngle := (360 - (angleDeg % 360)) % 360
	cosA := CosTable[safeAngle]
	sinA := SinTable[safeAngle]

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


		if c1.ShapeType == def.ShapeOBB && c2.ShapeType == def.ShapeBox {
		return checkOBBVsAABB(x1, y1, c1.Width, c1.Height, angle1, x2, y2, c2.Width, c2.Height)
	}
	if c1.ShapeType == def.ShapeBox && c2.ShapeType == def.ShapeOBB {
		return checkOBBVsAABB(x2, y2, c2.Width, c2.Height, angle2, x1, y1, c1.Width, c1.Height)
	}


	// Nếu rơi vào các trường hợp không cần thiết (VD: OBB vs OBB), tự động bỏ qua.
	return false 
}
func isPointInAABB(px, py float32, boxX, boxY, w, h float32) bool {
	halfW := w / 2.0
	halfH := h / 2.0
	return px >= boxX-halfW && px <= boxX+halfW &&
	       py >= boxY-halfH && py <= boxY+halfH
}
func isPointInOBB(px, py float32, rectX, rectY, w, h float32, angleDeg uint16) bool {
    dx := px - rectX
    dy := py - rectY

	safeAngle := (360 - (angleDeg % 360)) % 360
	cosA := CosTable[safeAngle]
	sinA := SinTable[safeAngle]


    localX := dx*cosA - dy*sinA
    localY := dx*sinA + dy*cosA

    halfW := w / 2.0
    halfH := h / 2.0

    return localX >= -halfW && localX <= halfW &&
           localY >= -halfH && localY <= halfH
}
func checkOBBVsAABB(obbX, obbY, obbW, obbH float32, obbAngle uint16, aabbX, aabbY, aabbW, aabbH float32) bool {
	// 1. LẤY 4 GÓC CỦA OBB
	safeAngle := obbAngle % 360
	cosA := CosTable[safeAngle]
	sinA := SinTable[safeAngle]
	
	halfW := obbW / 2.0
	halfH := obbH / 2.0
	
	corners := [4][2]float32{
		{-halfW, -halfH}, {halfW, -halfH},
		{halfW, halfH},   {-halfW, halfH},
	}

	// 2. KIỂM TRA TỪNG GÓC OBB CÓ LỌT VÀO AABB KHÔNG
	for _, c := range corners {
		// Xoay góc về tọa độ thế giới
		worldX := obbX + c[0]*cosA - c[1]*sinA
		worldY := obbY + c[0]*sinA + c[1]*cosA

		if isPointInAABB(worldX, worldY, aabbX, aabbY, aabbW, aabbH) {
			return true
		}
	}
    
    // 3. NGƯỢC LẠI: KIỂM TRA 4 GÓC AABB CÓ LỌT VÀO OBB KHÔNG
    // Điều này xử lý trường hợp OBB rất to nhưng không góc nào lọt vào AABB nhỏ
    halfAabbW := aabbW / 2.0
    halfAabbH := aabbH / 2.0
    aabbCorners := [4][2]float32{
        {aabbX - halfAabbW, aabbY - halfAabbH},
        {aabbX + halfAabbW, aabbY - halfAabbH},
        {aabbX + halfAabbW, aabbY + halfAabbH},
        {aabbX - halfAabbW, aabbY + halfAabbH},
    }
    for _, c := range aabbCorners {
        if isPointInOBB(c[0], c[1], obbX, obbY, obbW, obbH, obbAngle) {
            return true
        }
    }

	return false
}