package handlers

import (
	"fmt"
	"strconv"

	"github.com/chrisostomemataba/balceinv-api/services"
	"github.com/chrisostomemataba/balceinv-api/utils"
	"github.com/gofiber/fiber/v2"
)

type ProductHandler struct {
	service *services.ProductService
}

func NewProductHandler(service *services.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) GetAll(c *fiber.Ctx) error {
	search := c.Query("search")
	category := c.Query("category")

	products, err := h.service.GetAll(search, category)
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.Success(c, "Products fetched", products)
}

func (h *ProductHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Invalid product ID")
	}

	product, err := h.service.GetByID(uint(id))
	if err != nil {
		return utils.Error(c, fiber.StatusNotFound, err.Error())
	}
	return utils.Success(c, "Product fetched", product)
}

func (h *ProductHandler) Create(c *fiber.Ctx) error {
	var input services.CreateProductInput
	if err := c.BodyParser(&input); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	product, err := h.service.Create(input)
	if err != nil {
		status := fiber.StatusInternalServerError
		if err.Error() == "product with this SKU already exists" {
			status = fiber.StatusConflict
		}
		return utils.Error(c, status, err.Error())
	}
	return utils.Success(c, "Product created", product)
}

func (h *ProductHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Invalid product ID")
	}

	var input services.UpdateProductInput
	if err := c.BodyParser(&input); err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Invalid request body")
	}

	// Pass the authenticated user's ID for the price history record
	payload := c.Locals("user").(*utils.TokenPayload)
	userID := payload.UserID

	product, err := h.service.Update(uint(id), input, &userID)
	if err != nil {
		status := fiber.StatusInternalServerError
		if err.Error() == "product not found" {
			status = fiber.StatusNotFound
		}
		return utils.Error(c, status, err.Error())
	}
	return utils.Success(c, "Product updated", product)
}

func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "Invalid product ID")
	}

	if err := h.service.Delete(uint(id)); err != nil {
		status := fiber.StatusInternalServerError
		if err.Error() == "product not found" {
			status = fiber.StatusNotFound
		}
		return utils.Error(c, status, err.Error())
	}
	return utils.Success(c, "Product deleted", nil)
}

func (h *ProductHandler) UploadExcel(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "No file uploaded")
	}

	result, err := h.service.UploadExcel(file)
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.Success(c, fmt.Sprintf("Imported %d products", result.Created), result)
}

func (h *ProductHandler) GetTemplate(c *fiber.Ctx) error {
	data, err := h.service.GetTemplate()
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "Could not generate template")
	}

	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", "attachment; filename=products_template.xlsx")
	return c.Send(data)
}

func (h *ProductHandler) GetLowStock(c *fiber.Ctx) error {
	products, err := h.service.GetLowStock()
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, err.Error())
	}
	return utils.Success(c, "Low stock products", products)
}