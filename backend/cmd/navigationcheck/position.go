package main

import "github.com/DidierWalbrecque/routeur-virtual-regatta/backend/internal/navigation"

func parseVirtualRegattaPosition(
	value string,
) (navigation.Position, error) {
	return navigation.ParseVirtualRegattaPosition(value)
}

func formatVirtualRegattaPosition(
	position navigation.Position,
) string {
	result, err := navigation.FormatVirtualRegattaPosition(position)
	if err != nil {
		return ""
	}
	return result
}
