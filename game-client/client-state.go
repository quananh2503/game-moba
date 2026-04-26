package main

import (
	def "game/pkg"
	"image/color"
	"math"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	ebitenVector "github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	MapSize      = 4000.0
	PlayerRadius = 25.0
	TickRate     = 60
	VisionRadius = 500.0 // Tầm nhìn chuẩn của Player
)

// --- STRUCTS CƠ BẢN ---
type ClientWall struct{ X, Y, W, H float32 }
type ClientBush struct{ X, Y, Radius float32 }

type ClientPlayer struct {
	X, Y       float32
	HP         uint16
	StatusMask byte
}

type ClientProjectile struct {
	SpellID  def.Spell
	X, Y     float32
	Angle    uint16
	TimeLeft float32 // Dùng để chặn đạn nội suy lố
}

type ClientVFX struct {
	Type     def.VFXType
	X, Y     float32
	Angle    uint16
	TimeLeft float32
}

type PlayerInput struct {
	keys         uint8
	angle        uint16
	rangeToMouse uint16
}

type ClientGame struct {
	netState    *GameState
	MyID        uint32
	ZoneX, ZoneY, ZoneRad float32
	input       PlayerInput
	Walls       []ClientWall
	Bushes      []ClientBush
	Players     map[uint32]*ClientPlayer
	Projectiles map[uint32]*ClientProjectile
	VFXs        map[uint32]*ClientVFX // Xóa theo ID
}

// --- HÌNH ẢNH PRE-RENDER TỐI ƯU ---
var (
	whitePixel       *ebiten.Image
	bushImage        *ebiten.Image
	circleFilled     *ebiten.Image
	circleStroke     *ebiten.Image
	imgExplosionBase *ebiten.Image
	
	// Dành riêng cho Sương mù (Fog of War)
	fogScreen  *ebiten.Image
	visionHole *ebiten.Image
)

var globalDrawOp ebiten.DrawImageOptions
const BaseRadius = 100.0

func init() {
	whitePixel = ebiten.NewImage(1, 1)
	whitePixel.Fill(color.White)

	bushImage = ebiten.NewImage(200, 200)
	ebitenVector.FillCircle(bushImage, 100, 100, 100, color.RGBA{34, 100, 34, 200}, true)
	ebitenVector.StrokeCircle(bushImage, 100, 100, 100, 4.0, color.RGBA{0, 80, 0, 255}, true)

	circleFilled = ebiten.NewImage(int(BaseRadius*2), int(BaseRadius*2))
	ebitenVector.FillCircle(circleFilled, BaseRadius, BaseRadius, BaseRadius, color.White, true)

	circleStroke = ebiten.NewImage(int(BaseRadius*2), int(BaseRadius*2))
	ebitenVector.StrokeCircle(circleStroke, BaseRadius, BaseRadius, BaseRadius, 8.0, color.White, true)

	imgExplosionBase = ebiten.NewImage(int(BaseRadius*2), int(BaseRadius*2))
	ebitenVector.FillCircle(imgExplosionBase, BaseRadius, BaseRadius, BaseRadius, color.White, true)

	// --- KHỞI TẠO LAYER SƯƠNG MÙ ---
	fogScreen = ebiten.NewImage(1280, 720)
	// Tạo con dấu đục lỗ (Màu trắng tinh, cạnh mờ dần)
	visionHole = ebiten.NewImage(int(VisionRadius*2), int(VisionRadius*2))
	ebitenVector.FillCircle(visionHole, VisionRadius, VisionRadius, VisionRadius, color.White, true)
}

// --- HÀM VẼ CƠ BẢN TỐI ƯU GPU ---
func drawGPURect(screen *ebiten.Image, x, y, width, height float32, clr color.Color) {
	globalDrawOp.GeoM.Reset()
	globalDrawOp.GeoM.Scale(float64(width), float64(height))
	globalDrawOp.GeoM.Translate(float64(x), float64(y))
	globalDrawOp.ColorScale.Reset()
	globalDrawOp.ColorScale.ScaleWithColor(clr)
	screen.DrawImage(whitePixel, &globalDrawOp)
}

func drawGPUCircle(screen *ebiten.Image, cx, cy, radius float32, clr color.Color) {
	scale := float64(radius / BaseRadius)
	globalDrawOp.GeoM.Reset()
	globalDrawOp.GeoM.Scale(scale, scale)
	globalDrawOp.GeoM.Translate(float64(cx-radius), float64(cy-radius))
	globalDrawOp.ColorScale.Reset()
	globalDrawOp.ColorScale.ScaleWithColor(clr)
	screen.DrawImage(circleFilled, &globalDrawOp)
}

func drawGPUStrokeCircle(screen *ebiten.Image, cx, cy, radius float32, clr color.Color) {
	scale := float64(radius / BaseRadius)
	globalDrawOp.GeoM.Reset()
	globalDrawOp.GeoM.Scale(scale, scale)
	globalDrawOp.GeoM.Translate(float64(cx-radius), float64(cy-radius))
	globalDrawOp.ColorScale.Reset()
	globalDrawOp.ColorScale.ScaleWithColor(clr)
	screen.DrawImage(circleStroke, &globalDrawOp)
}

// ==========================================
// VÒNG LẶP LOGIC CLIENT
// ==========================================
func (g *ClientGame) Update() error {
	processNetworkEvents(g.netState, g)
	g.HandleInput()
	dt := float32(1.0 / 60.0)

	// 1. Cập nhật nội suy đạn
	for id, p := range g.Projectiles {
		p.TimeLeft -= dt
		if p.TimeLeft > 0 { // Còn sống thì bay
			speed := def.GetSpellData(p.SpellID).Speed
			rad := float64(p.Angle) * math.Pi / 180.0
			p.X += float32(math.Cos(rad)) * speed * dt
			p.Y += float32(math.Sin(rad)) * speed * dt
		} else if p.TimeLeft <= -0.2 {
			// Dự phòng: Lỡ mạng lag mất gói tin Xóa đạn, cho nó tự bay màu sau 0.2s
			delete(g.Projectiles, id)
		}
	}

	// 2. Cập nhật VFX
	for id, v := range g.VFXs {
		v.TimeLeft -= dt
		// An toàn: Xóa rác đồ họa lỡ Server quên gửi Event (Lag mạng)
		if v.TimeLeft <= -0.5 {
			delete(g.VFXs, id)
		}
	}

	sendAckEvents(g.netState, g)
	return nil
}

func (g *ClientGame) HandleInput() {
	var keys def.Input
	if ebiten.IsKeyPressed(ebiten.KeyW) { keys |= def.InputW }
	if ebiten.IsKeyPressed(ebiten.KeyA) { keys |= def.InputA }
	if ebiten.IsKeyPressed(ebiten.KeyD) { keys |= def.InputD }
	if ebiten.IsKeyPressed(ebiten.KeyS) { keys |= def.InputS }
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) { keys |= def.InputLeftClick }
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) { keys |= def.InputRightClick }
	if ebiten.IsKeyPressed(ebiten.KeySpace) { keys |= def.InputSpace }

	cx, cy := ebiten.CursorPosition()
	screenWidth, screenHeight := 1280.0, 720.0
	playerScreenX, playerScreenY := screenWidth/2.0, screenHeight/2.0

	rad := math.Atan2(float64(float32(cy)-float32(playerScreenY)), float64(float32(cx)-float32(playerScreenX)))
	deg := rad * (180.0 / math.Pi)
	if deg < 0 { deg += 360.0 }
	
	dist := float64(0)
	if keys.IsSet(def.InputLeftClick) || keys.IsSet(def.InputRightClick) {
		dx := float64(cx) - float64(playerScreenX)
		dy := float64(cy) - float64(playerScreenY)
		dist = math.Sqrt(dx*dx + dy*dy)
	}
	
	g.input = PlayerInput{
		keys:         uint8(keys),
		angle:        uint16(deg),
		rangeToMouse: uint16(dist),
	}
}

// ==========================================
// VÒNG LẶP VẼ ĐỒ HỌA
// ==========================================
func (g *ClientGame) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{44, 47, 51, 255}) // Nền đất

	var camX, camY float32 = 0, 0
	if me, ok := g.Players[uint32(g.MyID)]; ok {
		camX = me.X - 1280.0/2.0
		camY = me.Y - 720.0/2.0
	}
	
	padding := float32(200.0)
	camLeft, camRight := camX-padding, camX+1280.0+padding
	camTop, camBottom := camY-padding, camY+720.0+padding

	// 1. RÌA & LƯỚI BẢN ĐỒ
	drawGPURect(screen, 0-camX, 0-camY, MapSize, 5, color.Black)
	drawGPURect(screen, 0-camX, MapSize-camY, MapSize, 5, color.Black)
	drawGPURect(screen, 0-camX, 0-camY, 5, MapSize, color.Black)
	drawGPURect(screen, MapSize-camX, 0-camY, 5, MapSize+5, color.Black)

	for i := float32(0); i <= MapSize; i += 50 {
		drawGPURect(screen, i-camX, 0-camY, 2, MapSize, color.RGBA{255, 255, 255, 15})
		drawGPURect(screen, 0-camX, i-camY, MapSize, 2, color.RGBA{255, 255, 255, 15})
	}

	// 2. VẼ TƯỜNG VÀ BỤI CỎ
	for _, w := range g.Walls {
		if w.X+w.W/2 < camLeft || w.X-w.W/2 > camRight || w.Y+w.H/2 < camTop || w.Y-w.H/2 > camBottom { continue }
		drawX, drawY := w.X-w.W/2-camX, w.Y-w.H/2-camY
		drawGPURect(screen, drawX-5, drawY+w.H, w.W+10, 15, color.RGBA{0, 0, 0, 80})
		drawGPURect(screen, drawX, drawY, w.W, w.H, color.RGBA{90, 95, 100, 255})
	}

	for _, b := range g.Bushes {
		if b.X+b.Radius < camLeft || b.X-b.Radius > camRight || b.Y+b.Radius < camTop || b.Y-b.Radius > camBottom { continue }
		op := &ebiten.DrawImageOptions{}
		scale := float64(b.Radius / 100.0) 
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(float64(b.X-b.Radius-camX), float64(b.Y-b.Radius-camY))
		screen.DrawImage(bushImage, op)
	}

	// 3. VẼ ĐẠN
	for _, p := range g.Projectiles {
		if p.X < camLeft || p.X > camRight || p.Y < camTop || p.Y > camBottom { continue }
		drawX, drawY := p.X-camX, p.Y-camY

		if p.SpellID == def.SpellWindShear {
			DrawWindProjectile(screen, drawX, drawY, p.Angle)
			continue
		}		
		if p.SpellID == def.SpellShockwave {
			DrawEarthShockwave(screen, drawX, drawY, p.Angle)
			continue
		}

		c := color.RGBA{255, 100, 0, 255} 
		if p.SpellID == def.SpellToxicSpray { c = color.RGBA{150, 255, 100, 255} } else
		 if p.SpellID == def.SpellIceLance { c = color.RGBA{100, 220, 255, 255} }
		drawGPUCircle(screen, drawX, drawY, 12, c)
		drawGPUCircle(screen, drawX, drawY, 6, color.White)
	}

	// 4. VẼ VFX (Sử dụng Data Dictionary)
	for _, v := range g.VFXs {
		if v.X < camLeft-300 || v.X > camRight+300 || v.Y < camTop-300 || v.Y > camBottom+300 { continue }
		drawX, drawY := v.X-camX, v.Y-camY
		
		vfxData := def.GetVFXData(v.Type) // Lấy thông số từ điển chung

		if vfxData.Shape == def.VFXShapeCircle {
			switch v.Type {
			case def.VFXToxicCloud:     DrawToxicCloud(screen, drawX, drawY, vfxData.Radius, v.TimeLeft, vfxData.MaxTime)
			case def.VFXIceTrail:       DrawIceTrail(screen, drawX, drawY, vfxData.Radius, v.TimeLeft, vfxData.MaxTime)
			case def.VFXIceWarning:     DrawWarningArea(screen, drawX, drawY, vfxData.Radius, v.TimeLeft, vfxData.MaxTime, 0.4, 0.8, 1.0)
			case def.VFXBoulderWarning: DrawWarningArea(screen, drawX, drawY, vfxData.Radius, v.TimeLeft, vfxData.MaxTime, 0.8, 0.4, 0.1)
			case def.VFXTornado:        DrawTornado(screen, drawX, drawY, vfxData.Radius, v.TimeLeft, vfxData.MaxTime)
			default:                    DrawExplosionSprite(screen, drawX, drawY, vfxData.Radius, v.Type, v.TimeLeft, vfxData.MaxTime)
			}
		} else if vfxData.Shape == def.VFXShapeBox {
			DrawFlamewallVFX(screen, drawX, drawY, vfxData.W, vfxData.H, v.Angle, v.TimeLeft, vfxData.MaxTime)
		}
	}

	// 5. VẼ NGƯỜI CHƠI
	for id, p := range g.Players {
		if p.X < camLeft || p.X > camRight || p.Y < camTop || p.Y > camBottom { continue }
		
		col := color.RGBA{231, 76, 60, 255} // Địch màu đỏ
		if id == g.MyID {
			col = color.RGBA{25, 152, 219, 255} // Mình màu xanh
		}
		
		drawX, drawY := p.X-camX, p.Y-camY
		drawGPURect(screen, drawX-15, drawY+18, 30, 8, color.RGBA{0, 0, 0, 100}) // Bóng
		drawGPUCircle(screen, drawX, drawY, PlayerRadius, col)
		drawGPUStrokeCircle(screen, drawX, drawY, PlayerRadius, color.RGBA{0, 0, 0, 150})
		
		// Thanh máu
		hpWidth := float32(p.HP) / 3000.0 * 50.0
		drawGPURect(screen, drawX-26, drawY-41, 52, 8, color.RGBA{0, 0, 0, 200})
		hpColor := color.RGBA{46, 204, 113, 255}
		if p.HP < 1500 { hpColor = color.RGBA{241, 196, 15, 255} }
		if p.HP < 500 { hpColor = color.RGBA{231, 76, 60, 255} }
		drawGPURect(screen, drawX-25, drawY-40, hpWidth, 6, hpColor)
	}

	// 6. VẼ SƯƠNG MÙ (FOG OF WAR) - CHE MÀN HÌNH LẠI
	// g.DrawFogOfWar(screen)
}

// Hàm vẽ Sương mù cực đẹp
func (g *ClientGame) DrawFogOfWar(screen *ebiten.Image) {
	// Xóa sạch hình sương mù cũ, đổ màu đen nhạt bao phủ
	fogScreen.Clear()
	fogScreen.Fill(color.RGBA{10, 15, 20, 230}) // Đen xanh xám (Rất điện ảnh)

	// Đục lỗ sáng tại vị trí mình
	if _, ok := g.Players[uint32(g.MyID)]; ok {
		op := &ebiten.DrawImageOptions{}
		
		// BlendDestinationOut = Xóa pixel ở đâu có màu trắng chồng lên
		op.Blend = ebiten.BlendDestinationOut 
		
		// Đặt tâm vùng sáng giữa màn hình (vì Camera follow người chơi)
		op.GeoM.Translate(float64(1280.0/2.0 - VisionRadius), float64(720.0/2.0 - VisionRadius))
		
		fogScreen.DrawImage(visionHole, op)
	}

	// Vẽ tấm sương mù đục lỗ đè lên màn hình thật
	screen.DrawImage(fogScreen, nil)
}

// =====================================================================
// CÁC HÀM HIỆU ỨNG (Giữ nguyên như cũ, vì đã chuẩn rồi)
// =====================================================================
func DrawEarthShockwave(screen *ebiten.Image, x, y float32, angle uint16) {
	op := &ebiten.DrawImageOptions{}
	rad := float64(angle) * math.Pi / 180.0
	width, height := float64(80.0), float64(300.0) 

	op.GeoM.Scale(width+6, height+6)
	op.GeoM.Translate(-(width+6)/2, -(height+6)/2)
	op.GeoM.Rotate(rad)
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(0.2, 0.1, 0.05, 0.8)
	screen.DrawImage(whitePixel, op)

	op.GeoM.Reset()
	op.ColorScale.Reset()
	op.GeoM.Scale(width, height)
	op.GeoM.Translate(-width/2, -height/2)
	op.GeoM.Rotate(rad)
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(0.55, 0.27, 0.07, 1.0)
	screen.DrawImage(whitePixel, op)

	op.GeoM.Reset()
	op.ColorScale.Reset()
	op.GeoM.Scale(width*0.4, height*0.8)
	op.GeoM.Translate(-width*0.2, -height*0.4)
	op.GeoM.Rotate(rad)
	op.GeoM.Translate(float64(x), float64(y))
	op.ColorScale.Scale(0.7, 0.4, 0.15, 0.9)
	screen.DrawImage(whitePixel, op)
}

func DrawWarningArea(screen *ebiten.Image, x, y, radius float32, timeLeft, maxTime float32, r, g, b float32) {
	progress := 1.0 - (timeLeft / maxTime)
	scale := float64(radius / BaseRadius)
	
	opStroke := &ebiten.DrawImageOptions{}
	opStroke.GeoM.Scale(scale, scale)
	opStroke.GeoM.Translate(float64(x-radius), float64(y-radius))
	opStroke.ColorScale.Scale(r, g, b, 0.6)
	screen.DrawImage(circleStroke, opStroke)

	coreRad := radius * float32(progress)
	if coreRad <= 0 { return }
	
	coreScale := float64(coreRad / BaseRadius)
	opCore := &ebiten.DrawImageOptions{}
	opCore.GeoM.Scale(coreScale, coreScale)
	opCore.GeoM.Translate(float64(x-coreRad), float64(y-coreRad))
	
	alpha := float32(progress) * 0.7 
	opCore.ColorScale.Scale(r, g, b, alpha)
	screen.DrawImage(circleFilled, opCore)
}

func DrawToxicCloud(screen *ebiten.Image, x, y, radius float32, timeLeft, maxTime float32) {
	progress := 1.0 - (timeLeft / maxTime)
	pulse := float64(1.0 + math.Sin(float64(progress*20))*0.05)
	var alpha float32
	if progress < 0.1 { alpha = progress / 0.1 } else if progress > 0.8 { alpha = timeLeft / (maxTime * 0.2) } else { alpha = 1.0 }

	op := &ebiten.DrawImageOptions{}
	outerScale := float64(radius/BaseRadius) * pulse
	op.GeoM.Scale(outerScale, outerScale)
	op.GeoM.Translate(float64(x-radius*float32(pulse)), float64(y-radius*float32(pulse)))
	op.ColorScale.Scale(0.2, 0.8, 0.0, alpha*0.3)
	screen.DrawImage(circleFilled, op)

	opInner := &ebiten.DrawImageOptions{}
	innerPulse := 1.0 + math.Cos(float64(progress*15))*0.03
	innerScale := float64(radius/BaseRadius) * 0.6 * innerPulse
	innerRad := radius * 0.6 * float32(innerPulse)
	opInner.GeoM.Scale(innerScale, innerScale)
	opInner.GeoM.Translate(float64(x-innerRad), float64(y-innerRad))
	opInner.ColorScale.Scale(0.1, 1.0, 0.2, alpha*0.6) 
	screen.DrawImage(circleFilled, opInner)
}

func DrawFlamewallVFX(screen *ebiten.Image, x, y, w, h float32, angle uint16, timeLeft, maxTime float32) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(w), float64(h))
	op.GeoM.Translate(float64(-w/2), float64(-h/2))
	rad := float64(angle) * math.Pi / 180.0
	op.GeoM.Rotate(rad)
	op.GeoM.Translate(float64(x), float64(y))

	alpha := float32(1.0)
	if timeLeft < 0.5 { alpha = timeLeft / 0.5 }
	op.ColorScale.Scale(1.0, 0.4, 0.0, alpha)
	screen.DrawImage(whitePixel, op)
}

func DrawExplosionSprite(screen *ebiten.Image, x, y float32, radius float32, vfxType def.VFXType, timeLeft, maxTime float32) {
	if imgExplosionBase == nil || maxTime <= 0 { return }
	progress := 1.0 - (timeLeft / maxTime) 
	if progress < 0 { progress = 0 }

	op := &ebiten.DrawImageOptions{}
	w, h := float64(imgExplosionBase.Bounds().Dx()), float64(imgExplosionBase.Bounds().Dy())
	targetScale := float64(radius / BaseRadius)
	scaleEasing := 1.0 - math.Pow(1.0-float64(progress), 3)
	currentScale := targetScale * scaleEasing

	op.GeoM.Translate(-w/2, -h/2)
	op.GeoM.Scale(currentScale, currentScale)

	var alpha float32
	if progress < 0.2 { alpha = progress / 0.2 } else { alpha = 1.0 - ((progress - 0.2) / 0.8) }

	var r, g, b float32
	switch vfxType {
	case def.VFXFireExplosion: r, g, b = 1.0, 0.4, 0.1
	case def.VFXPoisonExplosion: r, g, b = 0.2, 0.8, 0.1
	case def.VFXIceExplosion: r, g, b = 0.3, 0.8, 1.0
	default: r, g, b = 1.0, 1.0, 1.0
	}

	op.ColorScale.Scale(r, g, b, alpha)
	op.GeoM.Translate(float64(x), float64(y))
	screen.DrawImage(imgExplosionBase, op)
}
func DrawIceTrail(screen *ebiten.Image, x, y, radius float32, timeLeft, maxTime float32) {
	// Tránh chia cho 0 lỡ maxTime bị lỗi
	if maxTime <= 0 { maxTime = 2.0 }
	
	// Alpha mặc định là 1.0 (Hiện rõ 100%)
	alpha := float32(1.0)
	
	// Chỉ làm mờ từ từ đi trong 0.5 giây cuối cùng trước khi biến mất
	if timeLeft < 0.5 {
		alpha = timeLeft / 0.5
	}

	// Lõi băng (Xanh lam nhạt)
	opBase := &ebiten.DrawImageOptions{}
	scale := float64(radius / BaseRadius)
	opBase.GeoM.Scale(scale, scale)
	opBase.GeoM.Translate(float64(x-radius), float64(y-radius))
	opBase.ColorScale.Scale(0.4, 0.8, 1.0, alpha*0.6) // Tăng độ sáng lên 0.6
	screen.DrawImage(circleFilled, opBase)

	// Lõi tâm (Sáng rực)
	opCore := &ebiten.DrawImageOptions{}
	coreScale := scale * 0.5 
	coreRad := radius * 0.5
	opCore.GeoM.Scale(coreScale, coreScale)
	opCore.GeoM.Translate(float64(x-coreRad), float64(y-coreRad))
	opCore.ColorScale.Scale(0.8, 0.95, 1.0, alpha)
	screen.DrawImage(circleFilled, opCore)
}

func DrawWindProjectile(screen *ebiten.Image, x, y float32, angle uint16) {
	rad := float64(angle) * math.Pi / 180.0
	opAura := &ebiten.DrawImageOptions{}
	auraRad := 25.0
	scaleAura := float64(auraRad / BaseRadius)
	opAura.GeoM.Scale(scaleAura, scaleAura)
	opAura.GeoM.Translate(float64(x-float32(auraRad)), float64(y-float32(auraRad)))
	opAura.ColorScale.Scale(0.4, 0.9, 1.0, 0.3) 
	screen.DrawImage(circleFilled, opAura)

	opBlade := &ebiten.DrawImageOptions{}
	scaleX, scaleY := 45.0/BaseRadius, 10.0/BaseRadius
	opBlade.GeoM.Scale(float64(scaleX), float64(scaleY))
	opBlade.GeoM.Translate(-45.0, -10.0)
	opBlade.GeoM.Rotate(rad)
	opBlade.GeoM.Translate(float64(x), float64(y))
	opBlade.ColorScale.Scale(0.9, 1.0, 1.0, 0.9) 
	screen.DrawImage(circleFilled, opBlade)
}

func DrawTornado(screen *ebiten.Image, x, y float32, radius float32, timeLeft, maxTime float32) {
	progress := 1.0 - (timeLeft / maxTime)
	radRotation := float64(progress * 15 * math.Pi) 
	alpha := float32(1.0)
	if timeLeft < 1.5 { alpha = timeLeft / 1.5 }

	opAura := &ebiten.DrawImageOptions{}
	scaleAura := float64(radius / BaseRadius) 
	opAura.GeoM.Scale(scaleAura, scaleAura)
	opAura.GeoM.Translate(float64(-radius), float64(-radius))
	opAura.GeoM.Rotate(radRotation * 0.2) 
	opAura.GeoM.Translate(float64(x), float64(y))
	opAura.ColorScale.Scale(0.7, 0.9, 1.0, alpha*0.1) 
	screen.DrawImage(circleStroke, opAura) 

	opCore := &ebiten.DrawImageOptions{}
	coreRad := radius * 0.4 
	scaleCore := float64(coreRad / BaseRadius)
	opCore.GeoM.Scale(scaleCore, scaleCore)
	opCore.GeoM.Translate(float64(-coreRad), float64(-coreRad))
	opCore.GeoM.Rotate(-radRotation * 1.5) 
	opCore.GeoM.Translate(float64(x), float64(y))
	opCore.ColorScale.Scale(0.4, 0.8, 0.9, alpha*0.4) 
	screen.DrawImage(circleFilled, opCore) 

	opEye := &ebiten.DrawImageOptions{}
	eyeRad := coreRad * 0.3
	scaleEye := float64(eyeRad / BaseRadius)
	opEye.GeoM.Scale(scaleEye, scaleEye)
	opEye.GeoM.Translate(float64(-eyeRad), float64(-eyeRad))
	opEye.GeoM.Rotate(radRotation * 3.0) 
	opEye.GeoM.Translate(float64(x), float64(y))
	opEye.ColorScale.Scale(0.8, 1.0, 1.0, alpha*0.8) 
	screen.DrawImage(circleStroke, opEye)
}

func (g *ClientGame) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 1280, 720
}