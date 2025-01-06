package main

type KalmanFilter struct {
	q float64 // Process noise covariance
	r float64 // Measurement noise covariance
	x float64 // Current estimate
	p float64 // Estimation error covariance
}

func NewKalmanFilter(q, r float64) *KalmanFilter {
	return &KalmanFilter{
		q: q,
		r: r,
		p: 1.0, // Initial estimation error
		x: 0.0, // Initial estimate
	}
}

// Update performs a Kalman filter update based on a new measurement
func (kf *KalmanFilter) Update(measurement float64) float64 {
	// Prediction step
	kf.p = kf.p + kf.q

	// Kalman gain calculation
	k := kf.p / (kf.p + kf.r)

	// Correction step
	kf.x = kf.x + k*(measurement-kf.x)
	kf.p = (1 - k) * kf.p

	return kf.x
}
