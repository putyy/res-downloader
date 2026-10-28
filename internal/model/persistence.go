package model

import "strings"

<<<<<<< Updated upstream
=======
// PersistentResourceCandidate returns a deep copy of candidate with every header
// listed in a track's NonPersistentHeaders removed, so short-lived credentials
// are not written to resources.db. The input value is left untouched.
>>>>>>> Stashed changes
func PersistentResourceCandidate(candidate ResourceCandidate) (ResourceCandidate, error) {
	persisted, err := CloneJSON(candidate)
	if err != nil {
		return ResourceCandidate{}, err
	}
	for index := range persisted.Tracks {
		track := &persisted.Tracks[index]
		for header := range track.Headers {
			for _, excluded := range track.NonPersistentHeaders {
				if strings.EqualFold(header, excluded) {
					delete(track.Headers, header)
					break
				}
			}
		}
	}
	return persisted, nil
}
