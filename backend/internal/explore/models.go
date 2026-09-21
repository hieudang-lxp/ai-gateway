package explore

import "context"

type ModelOption struct {
	Source string `json:"source"`
	Model  string `json:"model"`
}

func (s *Store) Models(ctx context.Context) ([]ModelOption, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT source,model FROM records
		WHERE source IN ('codex','claude_code','cursor') AND model<>'' ORDER BY source,model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := []ModelOption{}
	for rows.Next() {
		var option ModelOption
		if err := rows.Scan(&option.Source, &option.Model); err != nil {
			return nil, err
		}
		models = append(models, option)
	}
	return models, rows.Err()
}
