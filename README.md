## 简述

之前想要下载一些 landsat8 数据，但是苦于国内没有好的地方下载，只能去亚马逊云上下载。但是亚马逊上面没有提供检索的功能（或许是我没有找到）。

不过亚马逊上可以下载 `scene_list.gz` ，这个文件提供了所有 landsat8 影像数据的相关信息，包括下载地址。

但是这个数据里面没有每景影像的原始边界范围，只有外包框范围，于是又找到 WRS2 的相关数据。写了个简单的程序来导入到 MySQL ，以便进行比较方便的影像检索。

做了一个简单的查询页面，有兴趣的可以 [点进去看看](http://unispace-x.demo.sstir.cn:8001/landsat/web/) 。在地图上点击鼠标左键，即可查询当前点击位置的 landsat8 影像数据。

## 数据下载

- 完整的 WRS2 数据 [点击下载](WRS2_descending_0.7z)
- 去除南北极的 WRS2 数据 [点击下载](WRS2_descending_去除极地.7z)
- scene_list.gz [点击下载](https://landsat-pds.s3.amazonaws.com/c1/L8/scene_list.gz)