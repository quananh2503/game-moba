package main
func RunSystem1[A any](engine *ArchEngine, logic func(count int, entities []Entity, compA []A)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	for sig, arch := range engine.Archetypes {
		if (sig & maskA) == maskA {
			sliceA := arch.Columns[colA].(*[]A)
			logic(len(arch.Entities), arch.Entities, *sliceA)
		}
	}
}
func RunSystem1Ex[A any](engine *ArchEngine,excludeMask ComponentMask, logic func(count int, entities []Entity, compA []A)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	for sig, arch := range engine.Archetypes {
		if (sig & maskA) == maskA && (sig & excludeMask) ==0 {
			sliceA := arch.Columns[colA].(*[]A)
			logic(len(arch.Entities), arch.Entities, *sliceA)
		}
	}
}
func RunSystem2Ex[A any, B any](engine *ArchEngine, excludeMask ComponentMask, logic func(count int, entities []Entity, compA []A, compB []B)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	maskB := GetMask[B]()
	colB := GetColumnIdx(maskB)
	reqMask := maskA | maskB

	for sig, arch := range engine.Archetypes {
		// BÍ QUYẾT LÀ ĐÂY: Thêm điều kiện (sig & excludeMask) == 0
		if (sig&reqMask) == reqMask && (sig&excludeMask) == 0 {
			sliceA := *(arch.Columns[colA]).(*[]A)
			sliceB := *(arch.Columns[colB]).(*[]B)
			logic(len(arch.Entities), arch.Entities, sliceA, sliceB)
		}
	}
}
func RunSystem2[A any, B any](engine *ArchEngine, logic func(count int,entities []Entity, compA []A, compB []B)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	maskB := GetMask[B]()
	colB := GetColumnIdx(maskB)
	reqMask := maskA | maskB
	for sig, arch := range engine.Archetypes {
		if (sig & reqMask) == reqMask {
			sliceA := *(arch.Columns[colA]).(*[]A)
			sliceB := *(arch.Columns[colB]).(*[]B)
			count := len(arch.Entities)
			logic(count,arch.Entities, sliceA, sliceB)
		}
	}
}
func RunSystem3[A any, B any,C any ](engine *ArchEngine, logic func(count int,entities []Entity, compA []A, compB []B, compC[]C)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	maskB := GetMask[B]()
	colB := GetColumnIdx(maskB)
	maskC := GetMask[C]()
	colC := GetColumnIdx(maskC)
	reqMask := maskA | maskB |maskC
	for sig, arch := range engine.Archetypes {
		if (sig & reqMask) == reqMask {
			sliceA := *(arch.Columns[colA]).(*[]A)
			sliceB := *(arch.Columns[colB]).(*[]B)
			sliceC := *(arch.Columns[colC]).(*[]C)
			count := len(arch.Entities)
			logic(count,arch.Entities, sliceA, sliceB,sliceC)
		}
	}
}
func RunSystem3Ex[A any, B any,C any ](engine *ArchEngine,exclude ComponentMask, logic func(count int,entities []Entity, compA []A, compB []B, compC[]C)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	maskB := GetMask[B]()
	colB := GetColumnIdx(maskB)
	maskC := GetMask[C]()
	colC := GetColumnIdx(maskC)
	reqMask := maskA | maskB |maskC
	for sig, arch := range engine.Archetypes {
		if (sig & reqMask) == reqMask  && (sig & exclude) ==0{
			sliceA := *(arch.Columns[colA]).(*[]A)
			sliceB := *(arch.Columns[colB]).(*[]B)
			sliceC := *(arch.Columns[colC]).(*[]C)
			count := len(arch.Entities)
			logic(count,arch.Entities, sliceA, sliceB,sliceC)
		}
	}
}
func RunSystem4Ex[A any, B any,C any , D any](engine *ArchEngine,exclude ComponentMask, logic func(count int,entities []Entity, compA []A, compB []B, compC []C,comD []D)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	maskB := GetMask[B]()
	colB := GetColumnIdx(maskB)
	maskC := GetMask[C]()
	colC := GetColumnIdx(maskC)
	maskD := GetMask[D]()
	colD := GetColumnIdx(maskD)
	reqMask := maskA | maskB |maskC | maskD
	for sig, arch := range engine.Archetypes {
		if (sig & reqMask) == reqMask  && (sig & exclude) ==0{
			sliceA := *(arch.Columns[colA]).(*[]A)
			sliceB := *(arch.Columns[colB]).(*[]B)
			sliceC := *(arch.Columns[colC]).(*[]C)
			sliceD := *(arch.Columns[colD]).(*[]D)
			count := len(arch.Entities)
			logic(count,arch.Entities, sliceA, sliceB,sliceC,sliceD)
		}
	}
}
func RunSystem5Ex[A any, B any,C any ,D any,E any](engine *ArchEngine,exclude ComponentMask, logic func(count int,entities []Entity, compA []A, compB []B, compC[]C, compD []D , compE []E)) {
	maskA := GetMask[A]()
	colA := GetColumnIdx(maskA)
	maskB := GetMask[B]()
	colB := GetColumnIdx(maskB)
	maskC := GetMask[C]()
	colC := GetColumnIdx(maskC)
	maskD := GetMask[D]()
	colD := GetColumnIdx(maskD)
	maskE := GetMask[E]()
	colE := GetColumnIdx(maskE)
	reqMask := maskA | maskB |maskC |maskD | maskE

	for sig, arch := range engine.Archetypes {
		if (sig & reqMask) == reqMask &&(sig & exclude )==0 {
			sliceA := *(arch.Columns[colA]).(*[]A)
			sliceB := *(arch.Columns[colB]).(*[]B)
			sliceC := *(arch.Columns[colC]).(*[]C)
			sliceD := *(arch.Columns[colD]).(*[]D)
			sliceE := *(arch.Columns[colE]).(*[]E)
			count := len(arch.Entities)
			logic(count,arch.Entities, sliceA, sliceB,sliceC,sliceD,sliceE)
		}
	}
}