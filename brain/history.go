package brain

type ChatHistory []HistoryNode

type HistoryNode struct {
	User  Signed
	Model Signed
}

func (h *HistoryNode) Verify(sv SignVerifier) error {
	if err := h.User.Verify(sv); err != nil {
		return err
	}
	return h.Model.Verify(sv)
}

func (ch ChatHistory) Verify(sv SignVerifier) error {
	for i := range ch {
		if err := ch[i].Verify(sv); err != nil {
			return err
		}
	}
	return nil
}

func (ch *ChatHistory) AppendInPlace(user, model Signed) {
	*ch = append(*ch, HistoryNode{User: user, Model: model})
}
