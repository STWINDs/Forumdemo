package mysql

import "github.com/your-username/forum/internal/model"

func CreateVideo(video *model.Video) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `insert into videos(user_id, title, file_name, size, url) values(?,?,?,?,?)`
	_, err = db.Exec(sqlStr, video.UserID, video.Title, video.FileName, video.Size, video.URL)
	return
}

func DeleteVideo(id, userID int64) (err error) {
	if err = isReady(); err != nil { return }
	sqlStr := `delete from videos where id = ? and user_id = ?`
	_, err = db.Exec(sqlStr, id, userID)
	return
}

func GetVideoByID(id int64) (video *model.Video, err error) {
	if err = isReady(); err != nil { return }
	video = new(model.Video)
	sqlStr := `select id, user_id, title, file_name, size, url, create_time from videos where id = ?`
	err = db.Get(video, sqlStr, id)
	return
}
