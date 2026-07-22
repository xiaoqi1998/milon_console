package types

// NetworkSwitchRequest is the request body for switching the active network.
type NetworkSwitchRequest struct {
	Network string `json:"network" binding:"required"`
}

// PaginationQuery holds common pagination parameters.
type PaginationQuery struct {
	Limit  int `json:"limit" form:"limit"`
	Offset int `json:"offset" form:"offset"`
}
