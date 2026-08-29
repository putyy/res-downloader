package model

import "strings"

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
