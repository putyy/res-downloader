package resource

import shared "res-downloader/internal/model"

func resourceViewFromCandidate(candidate shared.ResourceCandidate) shared.ResourceView {
	return shared.ResourceView{ResourceCandidate: candidate}
}
