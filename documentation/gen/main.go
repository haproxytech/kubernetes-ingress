package main

func main() {
	var cn Conf
	cn.getConf()

	cn.generateReadme()
	cn.generateReadmeAnnotations()
	cn.generateReadmeController()
	cn.generateSupport()
	// cn.saveConf()
	// cn.saveDocConf()
}
