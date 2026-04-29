package physics

import "math"

// Vector3 is a point or direction in 3D space.
type Vector3 struct {
	X, Y, Z float64
}

func (v Vector3) Add(u Vector3) Vector3 {
	return Vector3{v.X + u.X, v.Y + u.Y, v.Z + u.Z}
}

func (v Vector3) Sub(u Vector3) Vector3 {
	return Vector3{v.X - u.X, v.Y - u.Y, v.Z - u.Z}
}

func (v Vector3) Scale(s float64) Vector3 {
	return Vector3{v.X * s, v.Y * s, v.Z * s}
}

func (v Vector3) LengthSq() float64 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

func (v Vector3) Length() float64 {
	return math.Sqrt(v.LengthSq())
}

// Normalize returns the unit vector; returns zero vector if length is zero.
func (v Vector3) Normalize() Vector3 {
	l := v.Length()
	if l == 0 {
		return Vector3{}
	}
	return v.Scale(1 / l)
}

func (v Vector3) Dot(u Vector3) float64 {
	return v.X*u.X + v.Y*u.Y + v.Z*u.Z
}

func (v Vector3) Cross(u Vector3) Vector3 {
	return Vector3{
		v.Y*u.Z - v.Z*u.Y,
		v.Z*u.X - v.X*u.Z,
		v.X*u.Y - v.Y*u.X,
	}
}
