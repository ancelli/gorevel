package controllers

import (
	"fmt"

	"github.com/robfig/revel"

	"gorevel/app/models"
	"gorevel/app/routes"
)

type User struct {
	Application
}

func (c User) Signup() revel.Result {
	return c.Render()
}

func (c User) SignupPost(user models.User) revel.Result {
	user.Validate(c.Validation)
	if c.Validation.HasErrors() {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect(routes.User.Signup())
	}

	user.Type = MEMBER_GROUP
	user.Avatar = models.DefaultAvatar
	user.ValidateCode = uuidName()
	user.HashedPassword = models.EncryptPassword(user.Password)

	aff, _ := engine.Insert(&user)
	if aff == 0 {
		c.Flash.Error("注册用户失败")
		return c.Redirect(routes.User.Signup())
	}

	subject := "激活账号 —— Revel社区"
	content := `<h2><a href="http://gorevel.cn/user/validate/` + user.ValidateCode + `">激活账号</a></h2>`
	go sendMail(subject, content, []string{user.Email})

	c.Flash.Success(fmt.Sprintf("%s 注册成功，请到您的邮箱 %s 激活账号！", user.Name, user.Email))

	engine.Insert(&models.Permissions{
		UserId: user.Id,
		Perm:   MEMBER_GROUP,
	})

	return c.Redirect(routes.User.Signin())
}

func (c User) Signin() revel.Result {
	return c.Render()
}

func (c User) SigninPost(name, password string) revel.Result {
	c.Validation.Required(name).Message("请输入用户名")
	c.Validation.Required(password).Message("请输入密码")

	if c.Validation.HasErrors() {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect(routes.User.Signin())
	}

	var user models.User
	engine.Where("name = ? AND hashed_password = ?", name, models.EncryptPassword(password)).Get(&user)

	if user.Id == 0 {
		c.Validation.Keep()
		c.FlashParams()
		c.Flash.Out["user"] = name
		c.Flash.Error("用户名或密码错误")
		return c.Redirect(routes.User.Signin())
	}

	if !user.IsActive {
		c.Flash.Error(fmt.Sprintf("您的账号 %s 尚未激活，请到您的邮箱 %s 激活账号！", user.Name, user.Email))
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect(routes.User.Signin())
	}

	c.Session["user"] = name

	if preUrl, ok := c.Session["preUrl"]; ok {
		return c.Redirect(preUrl)
	}

	return c.Redirect(routes.App.Index())
}

func (c User) Signout() revel.Result {
	for k := range c.Session {
		delete(c.Session, k)
	}

	return c.Redirect(routes.App.Index())
}

func (c User) Edit() revel.Result {
	avatars := models.Avatars
	return c.Render(avatars)
}

func (c User) EditPost(avatar string) revel.Result {
	if c.Validation.HasErrors() {
		c.Validation.Keep()
		c.FlashParams()
		return c.Redirect(routes.User.Edit())
	}

	var user models.User
	has, _ := engine.Id(c.userId).Get(&user)
	if !has {
		return c.NotFound("用户不存在")
	}

	file, header, err := c.Request.FormFile("picture")
	if err == nil {
		defer file.Close()
		if ok := checkFileExt(c.Validation, header, imageExts, "picture", "Only image"); ok {
			fileName := uuidFileName(header.Filename)
			filePath := UPLOAD_PATH + fileName

			saveFile(&file, filePath)
			thumbFile(filePath)

			deleteFile(UPLOAD_PATH + user.Avatar)
			user.Avatar = fileName
		}
	} else if avatar != "" {
		deleteFile(UPLOAD_PATH + user.Avatar)
		user.Avatar = avatar
	}

	aff, _ := engine.Id(c.userId).Cols("avatar").Update(&user)
	if aff > 0 {
		c.Flash.Success("保存成功")
	} else {
		c.Flash.Error("保存失败")
	}

	return c.Redirect(routes.User.Edit())
}

func (c User) Validate(code string) revel.Result {
	var user models.User
	has, _ := engine.Where("code = ?", code).Get(&user)

	if !has {
		return c.NotFound("用户不存在或校验码错误")
	}

	user.IsActive = true
	engine.Cols("is_active").Update(&user)

	c.Flash.Success("您的账号成功激活，请登录！")

	return c.Redirect(routes.User.Signin())
}