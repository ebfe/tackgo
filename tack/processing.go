package tack

type Status int

const (
	UNPINNED Status = iota
	ACCEPTED
	REJECTED
)

func ProcessStore(store PinStore, tackExt *TackExtension, name string,
	currentTime uint32) (status Status, err error) {

	tackMatchesPin := [2]bool{}
	pinIsActive, pinMatchesTack, pinMatchesActiveTack := [2]bool{}, [2]bool{}, [2]bool{}

	// Check tack generations and update min_generations
	tackFingerprints := tackExt.GetKeyFingerprints()
	for t, tack := range tackExt.Tacks {
		minGeneration, ok := store.GetMinGeneration(tackFingerprints[t])
		if ok {
			if tack.Generation < minGeneration {
				return status, RevokedError{}
			} else if tack.MinGeneration > minGeneration {
				store.SetMinGeneration(tackFingerprints[t], tack.MinGeneration)
			}
		}
	}

	// Determine the store's status
	for p, pin := range store.GetPinPair(name) {
		if pin.endTime > currentTime {
			pinIsActive[p] = true
		}
		for t, _ := range tackExt.Tacks {
			if pin.fingerprint == tackFingerprints[t] {
				pinMatchesTack[p] = true
				pinMatchesActiveTack[p] = tackExt.IsActive(t)
				tackMatchesPin[t] = true
			}
		}
		if pinIsActive[p] {
			if !pinMatchesTack[p] {
				return REJECTED, nil // return immediately
			}
			status = ACCEPTED
		}
	}

	// Perform pin activation
	if store.GetPinActivation() {
		newPins := []*Pin{}
		madeChanges := false

		// Delete unmatched pins and activate matched pins with active tacks
		for p, pin := range store.GetPinPair(name) {
			if !pinMatchesTack[p] {
				madeChanges = true // Delete pin (by not appending to newPair)
			} else {
				endTime := pin.endTime
				if pinMatchesActiveTack[p] && currentTime > pin.initialTime {
					endTime = currentTime + (currentTime - pin.initialTime) - 1
					if endTime > currentTime+(30*24*60) {
						endTime = currentTime + (30 * 24 * 60)
					}
					if endTime != pin.endTime {
						madeChanges = true // Activate pin
					}
				}
				// Append old pin to newPair, possibly extending endTime
				if len(newPins) > 1 {
					panic("ASSERT: only 2 pins allowed in pair")
				}
				newPins = append(newPins, &Pin{pin.initialTime, endTime, pin.fingerprint})
			}
		}

		// Add new inactive pins for any unmatched active tacks
		for t, tack := range tackExt.Tacks {
			if tackExt.IsActive(t) && !tackMatchesPin[t] {
				store.SetMinGeneration(tackFingerprints[t], tack.MinGeneration)
				if len(newPins) > 1 {
					panic("ASSERT: only 2 pins allowed in pair")
				}
				newPins = append(newPins, &Pin{currentTime, 0, tackFingerprints[t]})
				madeChanges = true // Add pin
			}
		}
		// Commit pin changes
		if madeChanges {
			store.SetPinPair(name, newPins)
		}
	}
	return status, err
}
