package libgorrent

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bencode "github.com/jackpal/bencode-go"
)

// File TODO
type File struct {
	Length  int64
	RawPath []string `bencode:"path"`
	Path    string
}

// TorrentFile TODO
type TorrentFile struct {
	// cadena que representa la URL del rastreador
	Announce string
	// (lista de cadenas opcional). Se usa para representar listas de rastreadores alternativos.
	// Es una extensión a la especificación original.
	RawAnnounceList [][]string `bencode:"announce-list"`
	AnnounceList    []string
	// (entero opcional) La fecha de creación del torrent en formato de época UNIX.
	RawCreationDate int64 `bencode:"creation date"`
	CreationDate    time.Time
	// (cadena opcional) Campo libre para el creador del torrent.
	Comment string
	// (cadena opcional) Nombre y versión del programa usado para crear el archivo torrent.
	CreatedBy string `bencode:"created-by"`

	InfoHash []byte
	Info     struct {
		// (cadena) El nombre del archivo o directorio donde se almacenarán los archivos.
		Name string
		// Como dijimos en la introducción, el archivo que queremos compartir es dividido en piezas.
		// Este parámetro es un entero que representa el número de bytes de cada pieza.
		// Piezas demasiado grandes causan ineficiencia y piezas demasiado pequeñas forman un archivo .torrent más pesado.
		// Actualmente se aconseja fijar el tamaño de cada pieza en 512 KB o menos para archivos de varios GBs.
		PieceLength int `bencode:"piece length"`
		// Cadena que representa la concatenación de la lista de claves hash de cada parte del fichero compartido.
		// Las claves hash son generadas utilizando SHA-1 con un resumen de 160 bits y un tamaño máximo por parte de 2^64 bits.
		// Este conjunto de claves se utiliza como mecanismo para asegurar la integridad y consistencia de una parte, una vez ha sido completada la descarga de dicha parte.
		AllPieces string `bencode:"pieces"`
		Pieces    [][]byte
		// (opcional). Es un entero que puede tener valores 0 ó 1 y que indica si se pueden buscar pares fuera de los rastreadores explícitamente descritos en la metainformación o no.
		Private bool
		// (entero) Longitud del archivo en bytes.
		Length int64
		// (cadena opcional). Es una cadena hexadecimal de 32 caracteres correspondiente a la suma MD5 del archivo.
		Md5sum string

		// Sólo aparecerá en el caso de que sea un torrent multi archivo. Es una lista de diccionarios (uno para cada archivo, pero con una estructura diferente a info).
		// Cada uno de estos diccionarios contendrá a su vez información sobre la longitud del archivo, la suma MD5 y una ruta (path) en donde debe ubicarse el archivo en la jerarquía de directorios.
		Files []File
	}
}

func appendIfMissing(slice []string, i string) []string {
	for _, ele := range slice {
		if ele == i {
			return slice
		}
	}
	return append(slice, i)
}

// GetLength TODO
func (t *TorrentFile) GetLength() int64 {
	if len(t.Info.Files) == 0 {
		// Single File
		return t.Info.Length
	}

	var sum int64
	for i := range t.Info.Files {
		sum = sum + t.Info.Files[i].Length
	}
	return sum
}

// GetFiles TODO
func (t *TorrentFile) GetFiles() []File {
	if len(t.Info.Files) == 0 {
		// Single File
		ret := make([]File, 0)
		ret = append(ret, File{
			Length: t.Info.Length,
			Path:   t.Info.Name,
		})
		return ret
	}

	return t.Info.Files
}

// LoadFromFile TODO
func LoadFromFile(fname string) (*TorrentFile, error) {
	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	torrent := TorrentFile{}
	err = bencode.Unmarshal(file, &torrent)
	if err != nil {
		return nil, err
	}

	// Process the pieces
	plen := len(torrent.Info.AllPieces) / sha1.Size
	torrent.Info.Pieces = make([][]byte, plen)
	for i := 0; i < plen; i++ {
		torrent.Info.Pieces[i] = []byte(torrent.Info.AllPieces[i*20 : (i+1)*20])
	}

	// Process the paths
	for i := range torrent.Info.Files {
		pathParts := append([]string{torrent.Info.Name}, torrent.Info.Files[i].RawPath...)
		torrent.Info.Files[i].Path = filepath.Join(pathParts...)
	}

	// Process the announcelist
	torrent.AnnounceList = appendIfMissing(torrent.AnnounceList, torrent.Announce)
	for i := range torrent.RawAnnounceList {
		torrent.AnnounceList = appendIfMissing(torrent.AnnounceList, torrent.RawAnnounceList[i][0])
	}

	torrent.CreationDate = time.Unix(torrent.RawCreationDate, 0)

	// torrent.Info.AllPieces = "*Removed*"

	// Obtengo el info-hash
	file.Seek(0, 0)
	data, _ := bencode.Decode(file)
	if err != nil {
		return nil, errors.New("Failed to decode torrent file: " + err.Error())
	}
	// fmt.Printf("%+v", data)

	torrentDict, ok := data.(map[string]interface{})
	if !ok {
		return nil, errors.New("Torrent file is not a dictionary")
	}

	infoBuffer := bytes.Buffer{}
	err = bencode.Marshal(&infoBuffer, torrentDict["info"])
	if err != nil {
		return nil, errors.New("Failed to encode info dict: " + err.Error())
	}

	hash := sha1.New()
	hash.Write(infoBuffer.Bytes())
	torrent.InfoHash = hash.Sum(nil)

	return &torrent, nil
}

// Debug TODO
func (t *TorrentFile) Debug() {
	fmt.Printf("Name: %s\n", t.Info.Name)
	fmt.Printf("Length: %d\n", t.Info.Length)
	fmt.Printf("Info Hash: %X\n", t.InfoHash)
	fmt.Printf("Files:\n")

	for i := range t.Info.Files {
		fmt.Printf("\t%+v (%d bytes)\n", t.Info.Files[i].Path, t.Info.Files[i].Length)
	}

	fmt.Printf("Announce: %s\n", t.Announce)
	fmt.Printf("AnnounceList:\n")

	for i := range t.AnnounceList {
		fmt.Printf("\t%+v\n", t.AnnounceList[i])
	}

	fmt.Printf("Comment: %s\n", t.Comment)
	fmt.Printf("CreatedBy: %s\n", t.CreatedBy)
	fmt.Printf("Creation Date: %s\n", t.CreationDate.UTC())
	fmt.Printf("PieceLength: %d\n", t.Info.PieceLength)
	fmt.Printf("Md5sum: %s\n", t.Info.Md5sum)

	fmt.Printf("Pieces:\n")

	for i := range t.Info.Pieces {
		if i >= 10 {
			fmt.Printf("\t... %d more\n", len(t.Info.Pieces)-10)
			break
		}

		fmt.Printf("\t%.2d: %X\n", i, t.Info.Pieces[i])
	}
}
