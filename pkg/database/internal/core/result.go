package core

type RowsAffectedResult struct {
	Count int64
}

func (r RowsAffectedResult) LastInsertId() (int64, error) {
	return 0, ErrLastInsertIDUnsupported
}

func (r RowsAffectedResult) RowsAffected() (int64, error) {
	return r.Count, nil
}
