# mbtileserver
tile server for mbtiles file

## creating mbtiles

1. Get osm.pbf ( gefabrick for instance )
2. Use tilemaker ```docker run --rm -it -v $(pwd):/srv tilemaker --input=/srv/<SOME AREA>.osm.pbf --output=/srv/<SOME AREA>.mbtiles```

## Some styling
![image](./example.png?raw=true)