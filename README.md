# This tool can extract, update, and list files in `.DAT` Higurashi Daybreak files

## Usage:

**Extracting files:**  
```bash
bundleToolsDaybreakGo.exe --extract input.DAT .\outputFolder\ filePatternOptional
```

**Overwriting:**  
```bash
bundleToolsDaybreakGo.exe --update input.DAT target.DAT
```

**Listing files inside the .DAT:**  
```bash
bundleToolsDaybreakGo.exe --list input.DAT
```

> ⚠️ Not finished and barely tested!

# How to Convert Daybreak `.X` Files to GLTF

1. **Open the Model in Fragmotion**  
   Download Fragmotion from: [http://www.fragmosoft.com/fragMOTION/index.php](http://www.fragmosoft.com/fragMOTION/index.php)  
   > ⚠️ Fragmotion may crash multiple times before successfully loading a model — just keep trying.

2. **(Optional) Remove Textures**  
   In the *Textures* tab, delete all textures.  
   > This step is recommended because Japanese characters in texture names can cause Blender not be able to import the files.

3. **Export as .X (Text Format)**  
   - Go to **File > Export**  
   - Choose the file type: `.x`  
   - Make sure the export is in **text format**, not binary.

4. **Fix Texture Filenames (if needed)**  
   If you didn’t remove the textures earlier, open the exported `.x` file in a text editor.  
   - Find all texture file names.  
   - Rename them to something that doesn't include Japanese characters.  
   - You can automate this step with a simple script.

5. **Convert `.X` to `.glTF` Using Assimp**  
   Use the following command:  
   ```bash
   assimp.exe export OriginalModel.x OutputModel.gltf
   ```

6. **Import into Blender**  
   You can now import the `.gltf` file into Blender.  
   - The model will include animations in the NLA tracks.  
   - However, animation timing may be incorrect (often too slow).  
   - You’ll need to adjust the keyframes manually in Blender.
