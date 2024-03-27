package data

import "gorm.io/gorm"

type Privilege struct {
	gorm.Model
	User      string
	Privilege int
}

func SetPrivilege(username string, priv int) error {
	return db.Where(Privilege{User: username}).Assign(Privilege{User: username, Privilege: priv}).FirstOrCreate(&Privilege{}).Error
}

func GetPrivilege(username string) (int, error) {
	var priv Privilege
	if err := db.Where("user = ?", username).First(&priv).Error; err != nil {
		return priv.Privilege, err
	}
	return priv.Privilege, nil
}
