// 这个项目用于将亚马逊上的 landsat8 影像列表数据
// 导入到 mysql 数据库（使用navicat导入也是很方便的）
// 导入的时候将每一个位置（path/row）的数据都从 wrs2 数据里面
// 查询了对应的地理范围。

package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type (
	landsatMeta struct {
		productId       string  // 产品ID
		entityId        string  // 标识ID
		acquisitionDate string  // 接收时间
		cloudCover      float32 // 云量 0-100
		processingLevel string  // 处理级别
		path            int     // 路径
		row             int     // 行
		min_lat         float32 // 最小纬度
		min_lon         float32 // 最小经度
		max_lat         float32 // 最大纬度
		max_lon         float32 // 最大经度
		downloadUrl     string  // 下载地址
	}
)

var (
	// WRS2 数据路径
	wrs2path string = "WRS2_descending_0/WRS2_descending.shp"

	// source_list 数据
	// 下载地址：https://landsat-pds.s3.amazonaws.com/c1/L8/scene_list.gz
	listfile string = "scene_list"
)

func main() {

	//============== 读取位置数据 ================
	loadWRS2Data(wrs2path)
	fmt.Println("数据加载完成,共读取:", len(sPathRowIndex))

	//============== 连接数据库 ================
	var uri = fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8", "root", "123456", "127.0.0.1", 3306)

	db, err := sql.Open("mysql", uri)
	if err != nil {
		fmt.Println("连接数据库错误:", err.Error())
		return
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		fmt.Println("无法访问数据库:", err.Error())
		return
	}
	if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS `landsat` DEFAULT CHARSET utf8 COLLATE utf8_general_ci"); err != nil {
		fmt.Println("无法创建数据库:", err.Error())
		return
	}
	if _, err = db.Exec("CREATE DATABASE IF NOT EXISTS `landsat` DEFAULT CHARSET utf8 COLLATE utf8_general_ci"); err != nil {
		fmt.Println("无法创建数据库:", err.Error())
		return
	}
	db.Exec("use landsat")

	createmetasql := "CREATE TABLE IF NOT EXISTS `meta`  (" +
		"  `productId` varchar(255) CHARACTER SET utf8 COLLATE utf8_bin NOT NULL," +
		"  `entityId` varchar(255) CHARACTER SET utf8 COLLATE utf8_bin NOT NULL," +
		"  `acquisitionDate` datetime(0) NOT NULL," +
		"  `cloudCover` float NOT NULL," +
		"  `processingLevel` varchar(4) CHARACTER SET utf8 COLLATE utf8_bin NOT NULL," +
		"  `path` int(11) NOT NULL," +
		"  `row` int(11) NOT NULL," +
		"  `minY` float NOT NULL," +
		"  `minX` float NOT NULL," +
		"  `maxY` float NOT NULL," +
		"  `maxX` float NOT NULL," +
		"  `url` varchar(767) CHARACTER SET utf8 COLLATE utf8_bin NOT NULL," +
		"  `bbox` geometry NOT NULL," +
		"  SPATIAL INDEX `bbox`(`bbox`)," +
		"  INDEX `path`(`path`) USING BTREE," +
		"  INDEX `row`(`row`) USING BTREE," +
		"  INDEX `cloudCover`(`cloudCover`) USING BTREE" +
		") ENGINE = InnoDB CHARACTER SET = utf8 COLLATE = utf8_bin ROW_FORMAT = Dynamic;"
	if _, err = db.Exec(createmetasql); err != nil {
		fmt.Println("无法创建数据库:", err.Error())
		return
	}
	//============== 读取数据 ================
	f, err := os.OpenFile(listfile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		fmt.Println("打开列表错误:", err.Error())
		return
	}
	defer f.Close()
	// 读取掉第一行
	buffer := bufio.NewReader(f)
	line, err := buffer.ReadBytes('\n')
	fmt.Println(string(line))

	tx, err := db.Begin()
	if err != nil {
		fmt.Println("开启事务失败", err.Error())
		return
	}
	var lineCount int = 0
	for {
		line, err = buffer.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("读取错误:", err.Error())
			return
		}
		// if skip -= 1; skip > 0 {
		// 	continue
		// }
		line = line[0 : len(line)-1] // 去除末尾 \n
		// productId,entityId,acquisitionDate,cloudCover,processingLevel,path,row,min_lat,min_lon,max_lat,max_lon,download_url
		// 名称,ID,时间,云量,处理级别,...页面链接
		parts := strings.Split(string(line), ",")
		if len(parts) != 12 {
			continue
		}
		path, _ := strconv.Atoi(parts[5])
		row, _ := strconv.Atoi(parts[6])
		obj := queryWRS2(path, row)
		if obj == nil {
			continue
		}
		wkt, err := polygonToWkt(obj.boundary)
		if err != nil {
			fmt.Println("边界出错:", path, row, err.Error())
			continue
		}

		insertsql := fmt.Sprintf("INSERT INTO `landsat`.`meta`(`productId`, `entityId`, `acquisitionDate`,"+
			" `cloudCover`, `processingLevel`, `path`, `row`, `minY`, `minX`, `maxY`, `maxX`, `url`,"+
			" `bbox`) VALUES ('%s', '%s', '%s', %s, '%s', %s, %s, %s, %s, %s, %s, '%s', ST_GeomFromText"+
			"('%s'));", parts[0], parts[1], parts[2], parts[3], parts[4], parts[5], parts[6],
			parts[7], parts[8], parts[9], parts[10], parts[11], wkt)
		// fmt.Println(insertsql)
		if _, err = tx.Exec(insertsql); err != nil {
			fmt.Println("插入错误:", err.Error())
			fmt.Println(lineCount, "行：", insertsql)
			break
		}
		lineCount += 1
		if lineCount%256 == 255 {
			fmt.Println("正在提交: ", lineCount-255, " 到 ", lineCount, " 行数据")
			err = tx.Commit()
			if err != nil {
				fmt.Println("提交数据失败:", err.Error())
				break
			}
			fmt.Println("事务提交完成")
			tx, _ = db.Begin()
		}
	}
	tx.Commit()
}
