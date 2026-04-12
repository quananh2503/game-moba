package main

import (
	"math/bits"
	"reflect"
)

type Entity uint32
type ComponentMask uint64 
var nextMask ComponentMask = 1
var compMasks = make(map[reflect.Type]ComponentMask)
type ComponentOpts struct{
	InitColumn func(arch *Archetype)
	MoveRow func(oldArch *Archetype,newArch *Archetype, oldRow int)
	SwapAndPop func(oldArch *Archetype,rowToRemove int)
	Remove func(arch *ArchEngine, entity Entity)
}
var compOpts= make([]ComponentOpts,64)
func GetColumnIdx(mask ComponentMask)int{
	return bits.TrailingZeros64(uint64(mask))
}
func GetMask[T any]()(ComponentMask){
	t := reflect.TypeOf((*T)(nil)).Elem()
	if mask,exists:=compMasks[t];exists{
		return mask
	}
	mask:=nextMask
	compMasks[t]= mask
	nextMask<<=1
	if nextMask==0{
		panic("qua 64 component")
	}
	colIdx := bits.TrailingZeros64(uint64(mask))
		compOpts[colIdx] = ComponentOpts{
		InitColumn: func(arch *Archetype) {
			slice := make([]T, 0)
			arch.Columns[colIdx] = &slice // Lưu con trỏ mảng vào any
		},
		MoveRow: func(oldArch *Archetype, newArch *Archetype, oldRow int) {
			// Ép kiểu 1 lần cực nhanh, không cần reflect
			oldSlice := oldArch.Columns[colIdx].(*[]T)
			newSlice := newArch.Columns[colIdx].(*[]T)
			*newSlice = append(*newSlice, (*oldSlice)[oldRow]) // Copy giá trị
			
		},
		SwapAndPop: func(arch *Archetype, row int) {
			slice := arch.Columns[colIdx].(*[]T)
			lastIdx := len(*slice) - 1
			(*slice)[row] = (*slice)[lastIdx] // Đem thằng cuối đè lên thằng bị xóa
			// Cắt bỏ đuôi (Shrink)
			var zero T
			(*slice)[lastIdx] = zero

			*slice = (*slice)[:lastIdx] 
		},
		Remove: func(arch *ArchEngine, entity Entity) {
			removeComponent[T](arch,entity)
		},
	}
	return mask
} 
const (
    IndexBits      = 24
    IndexMask      = (1 << IndexBits) - 1 
    GenerationMask = 0xFF000000          
)
func getIndex(e Entity) uint32 { 
	return uint32(e) & IndexMask 
}
func getGeneration(e Entity) uint8 { 
	return uint8(uint32(e) >> IndexBits) 
}
func makeEntity(index uint32, generation uint8) Entity {
    return Entity((uint32(generation) << IndexBits) | index)
}
type Archetype struct{
	Signature ComponentMask
	Entities  []Entity
	Columns [64]any
	
}
func NewArchetype(sig ComponentMask)*Archetype{
	return &Archetype{
		Signature: sig,
		Entities: make([]Entity,0),
		// Columns: ,
		
	}
}
type Record struct{
	Arch *Archetype
	Row int 
	Generation uint8
}
type ArchEngine struct{
	NextIndex uint32
	Registry []Record
	Archetypes map[ComponentMask]*Archetype
	// FrameEvents []RawEvent
	FreeIndices   []uint32
}
func NewArchEngine() *ArchEngine {
	w := &ArchEngine{
		NextIndex: 1,
		Registry:   make([]Record,100000),
		Archetypes: make(map[ComponentMask]*Archetype),
		// FrameEvents: make([]RawEvent, 0,500),
		FreeIndices: make([]uint32, 0,100000),
	}
	// Luôn tạo sẵn một "Bảng Trống" (Signature = 0) cho Entity mới sinh ra
	w.Archetypes[0] = NewArchetype(0)
	return w
}
func (e *ArchEngine)CreateEntity()Entity{
	var idx uint32
	if len(e.FreeIndices)>0{
		lastIdx := len(e.FreeIndices)-1
		idx = e.FreeIndices[lastIdx]
		e.FreeIndices=e.FreeIndices[:lastIdx]
	}else{
		idx = e.NextIndex
		e.NextIndex++
		if int(idx) >= len(e.Registry){
			newRegistry :=make([]Record, 2 * len(e.Registry))
			copy(newRegistry,e.Registry)
			e.Registry=newRegistry
		}
	}

	gen := e.Registry[idx].Generation
	entity := makeEntity(idx,gen)
	
	archZero := e.Archetypes[0]
	row := len(archZero.Entities)
	archZero.Entities=append(archZero.Entities, entity)
	e.Registry[idx].Arch=archZero
	e.Registry[idx].Row=row
	e.Registry[idx].Generation=gen

	return entity
}
func forAll(sig ComponentMask, logic func(mask ComponentMask, opt ComponentOpts)){

	for sig !=0{
		idx := bits.TrailingZeros64(uint64(sig))
		mask := ComponentMask(1<<idx)
		f := compOpts[idx]
		logic(mask,f)
		sig &=^mask
		
	}
}
func addComponent[T any](e *ArchEngine,entityID Entity, comp T){
	

	idx :=getIndex(entityID)
	if int(idx) >=len(e.Registry){
		return
	}
	gen := getGeneration(entityID)

	record:=e.Registry[idx]
	if gen != record.Generation{
		return
	}


	oldArch := record.Arch
	oldRow := record.Row
	maskT := GetMask[T]()
	if (oldArch.Signature & maskT) != 0{
		cols ,ok:= oldArch.Columns[GetColumnIdx(maskT)].(*[]T)
		if !ok{
			panic("Ep sai kieu ")
		}
		(*cols)[oldRow]=comp
		return 
	}
	newSig := oldArch.Signature|maskT
	newArch, exists := e.Archetypes[newSig]
	if !exists{
		newArch=NewArchetype(newSig)
		e.Archetypes[newSig]=newArch
		
		forAll(oldArch.Signature,func(mask ComponentMask, opt ComponentOpts) {
			opt.InitColumn(newArch)
		})
		f:=compOpts[bits.TrailingZeros64(uint64(maskT))]
		f.InitColumn(newArch)
	}
	newRow := len(newArch.Entities)
	newArch.Entities=append(newArch.Entities, entityID)
	forAll(oldArch.Signature,func(mask ComponentMask, opt ComponentOpts) {
		opt.MoveRow(oldArch,newArch,oldRow)
	})
	colIdx := bits.TrailingZeros64(uint64(maskT))
	sliceTo :=newArch.Columns[colIdx].(*[]T)
	(*sliceTo) = append((*sliceTo),comp)
	e.Registry[idx].Arch=newArch
	e.Registry[idx].Row=newRow


	
	forAll(oldArch.Signature,func(mask ComponentMask, opt ComponentOpts) {
		opt.SwapAndPop(oldArch,oldRow)
	})

	lastIdx :=len(oldArch.Entities)-1
	lastEntityID := oldArch.Entities[lastIdx]
	if oldRow!=lastIdx{
		(oldArch.Entities)[oldRow] = lastEntityID
		e.Registry[getIndex(lastEntityID)].Row=oldRow
	}
	(oldArch.Entities) = (oldArch.Entities)[:lastIdx]

}
func removeComponent[T any](e *ArchEngine,entityID Entity){
	idx := getIndex(entityID)
	gen :=getGeneration(entityID)
	record := e.Registry[idx]
	if gen != record.Generation{
		return
	}

	maskT :=GetMask[T]()
	oldArch :=record.Arch
	oldRow :=record.Row
	if (oldArch.Signature &maskT) ==0{
		return 
	}
	newSig := oldArch.Signature &^ maskT
	newArch ,exists:= e.Archetypes[newSig]
	
	if !exists{
		newArch = NewArchetype(newSig)
		e.Archetypes[newSig] = newArch

		forAll(oldArch.Signature,func(mask ComponentMask, opt ComponentOpts) {
			if mask == maskT{
				return
			}
			opt.InitColumn(newArch)
		})

	}
	newRow := len(newArch.Entities)
	newArch.Entities=append(newArch.Entities, entityID)
	e.Registry[idx].Arch=newArch
	e.Registry[idx].Row=newRow

	forAll(oldArch.Signature,func(mask ComponentMask, opt ComponentOpts) {
		if mask == maskT{
			return
		}
		opt.MoveRow(oldArch,newArch,oldRow)
	})
	lastIdx:= len(oldArch.Entities) - 1
	lastEntity := oldArch.Entities[lastIdx]
	forAll(oldArch.Signature,func(mask ComponentMask, opt ComponentOpts) {
		opt.SwapAndPop(oldArch,oldRow)
	})
	if lastIdx != oldRow{
		oldArch.Entities[oldRow]=lastEntity
		e.Registry[getIndex(lastEntity)].Row=oldRow
	}
	oldArch.Entities = oldArch.Entities[:lastIdx]

}
func (e *ArchEngine) RemoveEntity(entityID Entity) {
	idx := getIndex(entityID)
	gen :=getGeneration(entityID)
	record := e.Registry[idx]
	if gen != record.Generation{
		return 
	}
	e.Registry[idx].Generation++


	arch := record.Arch
	row := record.Row

	// 1. Lấy thông tin thằng cuối cùng trong Bảng
	lastIdx := len(arch.Entities) - 1
	lastEntityID := arch.Entities[lastIdx]

	// 2. Dọn dẹp tất cả các cột bằng Swap And Pop
	forAll(arch.Signature,func(mask ComponentMask, opt ComponentOpts) {
		opt.SwapAndPop(arch, row)
	})

	// 3. Lấp lỗ trống trong mảng Entities và cập nhật Registry
	if row != lastIdx {
		arch.Entities[row] = lastEntityID
		e.Registry[getIndex(lastEntityID)].Row=row
	}
	arch.Entities = arch.Entities[:lastIdx]

	e.FreeIndices=append(e.FreeIndices, idx)


}
func GetComponentByEntity[T any](engine *ArchEngine, entity Entity) (*T, bool) {
	idx := getIndex(entity)
	gen :=getGeneration(entity)
	if int(idx) >=len(engine.Registry) {
		return nil,false
	}
	r := engine.Registry[idx]

	if gen != r.Generation || r.Arch == nil {
		return nil, false
	}
	
	arch := r.Arch
	col := GetColumnIdx(GetMask[T]())
	slice, ok := (arch.Columns[col]).(*[]T)
	if ok && r.Row < len(*slice) {
		return &(*slice)[r.Row], true
	}
	return nil, false
}
func GetComponentByEntityAndMask[T any](engine *ArchEngine, entity Entity, mask ComponentMask)( *T,bool){
	idx := getIndex(entity)
	gen :=getGeneration(entity)
	if int(idx) >= len(engine.Registry) {
		return nil,false
	}
	r := engine.Registry[idx]

	if gen != r.Generation || r.Arch == nil   {
		return nil, false
	}
	
	arch := r.Arch
	colIdx:=GetColumnIdx(mask)
	slice, ok := (arch.Columns[colIdx]).(*[]T)
	if ok && r.Row < len(*slice) {
		return &(*slice)[r.Row], true
	}
	return nil, false	
}